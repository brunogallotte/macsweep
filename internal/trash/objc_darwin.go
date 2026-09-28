//go:build darwin

package trash

// Bridge to the handful of Objective-C calls macsweep needs, through purego
// rather than cgo. Staying cgo-free keeps CGO_ENABLED=0, which means both
// architecture slices cross-compile from a single arm64 runner. That matters
// because GitHub's x86_64 macOS runners are on their way out.

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	loadOnce sync.Once
	loadErr  error

	selAlloc                objc.SEL
	selInit                 objc.SEL
	selDrain                objc.SEL
	selDefaultManager       objc.SEL
	selStringWithUTF8String objc.SEL
	selFileURLWithPath      objc.SEL
	selPath                 objc.SEL
	selUTF8String           objc.SEL
	selLocalizedDescription objc.SEL
	selTrashItem            objc.SEL
	classNSAutoreleasePool  objc.Class
	classNSString           objc.Class
	classNSURL              objc.Class
	classNSFileManager      objc.Class
)

func load() error {
	loadOnce.Do(func() {
		_, err := purego.Dlopen(
			"/System/Library/Frameworks/Foundation.framework/Foundation",
			purego.RTLD_LAZY|purego.RTLD_GLOBAL,
		)
		if err != nil {
			loadErr = fmt.Errorf("carregando Foundation: %w", err)
			return
		}
		selAlloc = objc.RegisterName("alloc")
		selInit = objc.RegisterName("init")
		selDrain = objc.RegisterName("drain")
		selDefaultManager = objc.RegisterName("defaultManager")
		selStringWithUTF8String = objc.RegisterName("stringWithUTF8String:")
		selFileURLWithPath = objc.RegisterName("fileURLWithPath:")
		selPath = objc.RegisterName("path")
		selUTF8String = objc.RegisterName("UTF8String")
		selLocalizedDescription = objc.RegisterName("localizedDescription")
		selTrashItem = objc.RegisterName("trashItemAtURL:resultingItemURL:error:")
		classNSAutoreleasePool = objc.GetClass("NSAutoreleasePool")
		classNSString = objc.GetClass("NSString")
		classNSURL = objc.GetClass("NSURL")
		classNSFileManager = objc.GetClass("NSFileManager")
		if classNSFileManager == 0 {
			loadErr = errors.New("NSFileManager indisponivel")
		}
	})
	return loadErr
}

// pool wraps a batch in an autorelease pool. Every NSString and NSURL built
// below is autoreleased, so without this they would pile up for the lifetime
// of the process.
type pool struct{ id objc.ID }

func newPool() pool {
	return pool{id: objc.ID(classNSAutoreleasePool).Send(selAlloc).Send(selInit)}
}

func (p pool) release() {
	if p.id != 0 {
		p.id.Send(selDrain)
	}
}

func nsString(s string) (objc.ID, error) {
	b, err := syscall.BytePtrFromString(s)
	if err != nil {
		return 0, err
	}
	id := objc.ID(classNSString).Send(selStringWithUTF8String, unsafe.Pointer(b))
	runtime.KeepAlive(b)
	return id, nil
}

func goString(id objc.ID) string {
	if id == 0 {
		return ""
	}
	p := objc.Send[unsafe.Pointer](id, selUTF8String)
	if p == nil {
		return ""
	}
	var out []byte
	for i := 0; ; i++ {
		c := *(*byte)(unsafe.Pointer(uintptr(p) + uintptr(i)))
		if c == 0 {
			break
		}
		out = append(out, c)
	}
	return string(out)
}

// trashItem moves one path to the Trash and returns where it landed.
//
// The resulting URL matters: macOS renames on collision, so the returned name
// is frequently not the original one. It is the key the undo manifest is
// built on.
func trashItem(path string) (string, error) {
	if err := load(); err != nil {
		return "", err
	}

	p := newPool()
	defer p.release()

	str, err := nsString(path)
	if err != nil {
		return "", err
	}
	url := objc.ID(classNSURL).Send(selFileURLWithPath, str)
	if url == 0 {
		return "", fmt.Errorf("caminho invalido: %s", path)
	}

	mgr := objc.ID(classNSFileManager).Send(selDefaultManager)

	var resulting objc.ID
	var nsErr objc.ID
	ok := objc.Send[uint8](mgr, selTrashItem, url,
		unsafe.Pointer(&resulting), unsafe.Pointer(&nsErr))

	if ok == 0 {
		msg := goString(nsErr.Send(selLocalizedDescription))
		if msg == "" {
			msg = "motivo desconhecido"
		}
		return "", fmt.Errorf("%s: %s", path, msg)
	}

	return goString(resulting.Send(selPath)), nil
}
