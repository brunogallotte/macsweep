package scan

import (
	"sort"
	"sync"
	"time"
)

// Node is one entry in the scanned tree. Sizes are in bytes actually
// allocated on disk, never logical sizes.
type Node struct {
	Name    string
	Path    string
	IsDir   bool
	Own     int64 // bytes of this entry alone
	Size    int64 // Own plus everything beneath it, filled by aggregate
	Files   int64 // regular files at or below this node
	ModTime time.Time
	Err     error // set when the directory could not be read

	// Dataless marks an iCloud placeholder. Its bytes are not really here
	// and it must never be opened.
	Dataless bool
	// Restricted marks a file protected by SIP. Undeletable even as root, so
	// it is shown but never offered as reclaimable.
	Restricted bool

	Parent   *Node
	Children []*Node

	mu sync.Mutex
}

func (n *Node) addChild(c *Node) {
	c.Parent = n
	n.mu.Lock()
	n.Children = append(n.Children, c)
	n.mu.Unlock()
}

// aggregate sums sizes bottom up. It runs once, after the walk has finished
// and the tree is no longer being mutated, so it needs no locking.
func (n *Node) aggregate() {
	n.Size = n.Own
	if !n.IsDir {
		n.Files = 1
		return
	}
	for _, c := range n.Children {
		c.aggregate()
		n.Size += c.Size
		n.Files += c.Files
	}
}

// SortBySize orders children largest first, recursively. This is the order
// every view in macsweep presents, so it lives here rather than in the UI.
func (n *Node) SortBySize() {
	sort.Slice(n.Children, func(i, j int) bool {
		if n.Children[i].Size != n.Children[j].Size {
			return n.Children[i].Size > n.Children[j].Size
		}
		return n.Children[i].Name < n.Children[j].Name
	})
	for _, c := range n.Children {
		c.SortBySize()
	}
}

// Walk calls fn for this node and every node beneath it. Returning false
// stops the descent into that node's children.
func (n *Node) Walk(fn func(*Node) bool) {
	if !fn(n) {
		return
	}
	for _, c := range n.Children {
		c.Walk(fn)
	}
}

// Offenders returns the biggest directories, skipping the ones that merely
// pass their parent's bytes through.
//
// Without this, asking where the space went answers with a chain:
// Containers, then com.docker.docker, then Data, then vms, then 0, then
// data, all reporting the same 8.6 GB. Only the first two carry information.
// A directory holding nine tenths or more of its parent is a corridor, not a
// room, so it is walked through rather than listed.
func (n *Node) Offenders(count int) []*Node {
	var out []*Node
	n.Walk(func(c *Node) bool {
		if c == n || !c.IsDir {
			return true
		}
		if c.Parent != nil && c.Parent != n && c.Size*10 >= c.Parent.Size*9 {
			return true // corridor: keep descending, do not list it
		}
		out = append(out, c)
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	if len(out) > count {
		out = out[:count]
	}
	return out
}

// Top returns the n largest nodes anywhere in the tree that satisfy keep.
// It is how the "biggest offenders" digest is built.
func (n *Node) Top(count int, keep func(*Node) bool) []*Node {
	var out []*Node
	n.Walk(func(c *Node) bool {
		if keep(c) {
			out = append(out, c)
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Size > out[j].Size })
	if len(out) > count {
		out = out[:count]
	}
	return out
}
