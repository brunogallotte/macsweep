package scan

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
)

// Options controls a walk. The zero value is usable and scans a single
// filesystem without following symlinks.
type Options struct {
	// Workers is the number of directories read in parallel. Defaults to
	// twice the CPU count, because the walk is bound by metadata I/O rather
	// than by the CPU.
	Workers int

	// Skip is consulted for every entry before it is stat'ed. Returning true
	// leaves the entry out of the tree entirely.
	Skip func(path string, d fs.DirEntry) bool

	// CrossDevice allows descending into other mounted volumes. Off by
	// default, so scanning the home directory never wanders into an external
	// disk or a network share.
	CrossDevice bool

	// Progress, when set, is called periodically with the running totals.
	Progress func(Stats)
}

// Stats is the live progress of a walk.
type Stats struct {
	Files int64
	Dirs  int64
	Bytes int64
	Errs  int64

	// Denied counts entries the process was not allowed to read. It is kept
	// apart from Errs because it has a specific meaning on macOS: the
	// terminal lacks Full Disk Access, so the totals are an undercount. Any
	// number above zero means no total may be presented as authoritative.
	Denied int64
}

// Complete reports whether the walk saw everything it tried to see.
func (s Stats) Complete() bool { return s.Denied == 0 }

// Result is a finished walk.
type Result struct {
	Root  *Node
	Stats Stats
}

type walker struct {
	opts    Options
	seen    sync.Map // fileID -> struct{}, for hard link dedup
	rootDev int32
	files   atomic.Int64
	dirs    atomic.Int64
	bytes   atomic.Int64
	errs    atomic.Int64
	denied  atomic.Int64
	ctx     context.Context
}

// Walk scans root and returns the aggregated tree.
func Walk(ctx context.Context, root string, opts Options) (*Result, error) {
	root = filepath.Clean(root)
	fi, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}

	// A parallel walk holds a directory handle per worker. The soft limit is
	// 256 in many shells, which produces nondeterministic EMFILE failures, so
	// raise it to the hard limit up front.
	raiseFileLimit()

	workers := opts.Workers
	if workers <= 0 {
		// The walk waits on metadata I/O far more than it computes, so it
		// oversubscribes the CPU on purpose. Measured on APFS, a single
		// threaded walk sits near 34% CPU.
		workers = runtime.NumCPU() * 4
	}

	info := inspect(fi)
	w := &walker{opts: opts, rootDev: info.id.dev, ctx: ctx}

	rootNode := &Node{
		Name:    filepath.Base(root),
		Path:    root,
		IsDir:   fi.IsDir(),
		Own:     info.size,
		ModTime: info.modTime,
	}

	if !fi.IsDir() {
		rootNode.aggregate()
		return &Result{Root: rootNode, Stats: w.stats()}, nil
	}

	p := newPool()
	p.push(rootNode)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n, ok := p.pop()
				if !ok {
					return
				}
				if ctx.Err() == nil {
					w.readDir(n, p)
				}
				p.done()
			}
		}()
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	rootNode.aggregate()
	return &Result{Root: rootNode, Stats: w.stats()}, nil
}

func (w *walker) stats() Stats {
	return Stats{
		Files:  w.files.Load(),
		Dirs:   w.dirs.Load(),
		Bytes:  w.bytes.Load(),
		Errs:   w.errs.Load(),
		Denied: w.denied.Load(),
	}
}

func (w *walker) countErr(err error) {
	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
		w.denied.Add(1)
		return
	}
	w.errs.Add(1)
}

func (w *walker) readDir(n *Node, p *pool) {
	w.dirs.Add(1)

	f, err := os.Open(n.Path)
	if err != nil {
		n.Err = err
		w.countErr(err)
		return
	}
	// ReadDir(-1) on the open file returns entries unsorted, which is what we
	// want: sorting here would be wasted work, the tree is sorted once at the
	// end by size.
	entries, err := f.ReadDir(-1)
	f.Close()
	if err != nil {
		n.Err = err
		w.countErr(err)
		return
	}

	for _, e := range entries {
		if w.opts.Skip != nil && w.opts.Skip(filepath.Join(n.Path, e.Name()), e) {
			continue
		}

		fi, err := e.Info() // lstat: a symlink is measured, never followed
		if err != nil {
			w.countErr(err)
			continue
		}
		info := inspect(fi)

		child := &Node{
			Name:       e.Name(),
			Path:       filepath.Join(n.Path, e.Name()),
			IsDir:      e.IsDir(),
			Own:        info.size,
			ModTime:    info.modTime,
			Dataless:   info.dataless(),
			Restricted: info.restricted(),
		}

		// An iCloud placeholder holds no local bytes. Record it, count
		// nothing, and never descend into it.
		if child.Dataless {
			child.Own = 0
			n.addChild(child)
			continue
		}

		switch {
		case e.IsDir():
			// A directory on another device is a mount point. Record it so it
			// is visible in the tree, but do not descend into it.
			if !w.opts.CrossDevice && info.ok && info.id.dev != w.rootDev {
				child.Own = 0
				n.addChild(child)
				continue
			}
			n.addChild(child)
			p.push(child)
			continue

		case fi.Mode().IsRegular():
			// A file with several hard links must only be counted once, or a
			// Homebrew Cellar or a node_modules with hoisted links inflates
			// the total.
			if info.ok && info.links > 1 {
				if _, dup := w.seen.LoadOrStore(info.id, struct{}{}); dup {
					child.Own = 0
				}
			}
			w.files.Add(1)
		}

		w.bytes.Add(child.Own)
		n.addChild(child)
	}

	if w.opts.Progress != nil {
		w.opts.Progress(w.stats())
	}
}

// raiseFileLimit lifts RLIMIT_NOFILE to the hard limit. Failure is not fatal:
// the walk still runs, it just risks EMFILE on very wide trees.
func raiseFileLimit() {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return
	}
	if lim.Cur >= lim.Max {
		return
	}
	lim.Cur = lim.Max
	_ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim)
}

// pool is a LIFO queue of directories waiting to be read. LIFO keeps the walk
// depth first, which keeps the number of live nodes low on a deep tree.
type pool struct {
	mu      sync.Mutex
	cond    *sync.Cond
	stack   []*Node
	pending int
}

func newPool() *pool {
	p := &pool{}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// push adds a directory and counts it as outstanding work. Children are
// always pushed before their parent calls done, so pending only reaches zero
// once the whole tree has been read.
func (p *pool) push(n *Node) {
	p.mu.Lock()
	p.stack = append(p.stack, n)
	p.pending++
	p.mu.Unlock()
	p.cond.Signal()
}

func (p *pool) pop() (*Node, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for len(p.stack) == 0 && p.pending > 0 {
		p.cond.Wait()
	}
	if len(p.stack) == 0 {
		return nil, false
	}
	n := p.stack[len(p.stack)-1]
	p.stack = p.stack[:len(p.stack)-1]
	return n, true
}

func (p *pool) done() {
	p.mu.Lock()
	p.pending--
	last := p.pending == 0
	p.mu.Unlock()
	if last {
		p.cond.Broadcast()
	}
}
