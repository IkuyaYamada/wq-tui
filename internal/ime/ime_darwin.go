package ime

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const cfStringEncodingUTF8 = 0x08000100

// tis binds the Carbon Text Input Sources API at runtime through purego, so
// the binary builds without cgo.
type tis struct {
	getCString func(s uintptr, buf *byte, n int, enc uint32) bool
	release    func(uintptr)
	arrCount   func(uintptr) int
	arrAt      func(uintptr, int) uintptr
	current    func() uintptr
	ascii      func() uintptr
	property   func(src, key uintptr) uintptr
	list       func(filter uintptr, includeAll bool) uintptr
	selectSrc  func(uintptr) int32
	keyID      uintptr
}

var (
	loadOnce sync.Once
	api      *tis
)

func load() *tis {
	loadOnce.Do(func() {
		cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		carbon, err := purego.Dlopen("/System/Library/Frameworks/Carbon.framework/Carbon", purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		keyID, err := purego.Dlsym(carbon, "kTISPropertyInputSourceID")
		if err != nil {
			return
		}
		// keyID is the address of a global CFStringRef; read the pointer it
		// holds. Going through &keyID keeps vet's uintptr check happy.
		t := &tis{keyID: **(**uintptr)(unsafe.Pointer(&keyID))}
		purego.RegisterLibFunc(&t.getCString, cf, "CFStringGetCString")
		purego.RegisterLibFunc(&t.release, cf, "CFRelease")
		purego.RegisterLibFunc(&t.arrCount, cf, "CFArrayGetCount")
		purego.RegisterLibFunc(&t.arrAt, cf, "CFArrayGetValueAtIndex")
		purego.RegisterLibFunc(&t.current, carbon, "TISCopyCurrentKeyboardInputSource")
		purego.RegisterLibFunc(&t.ascii, carbon, "TISCopyCurrentASCIICapableKeyboardInputSource")
		purego.RegisterLibFunc(&t.property, carbon, "TISGetInputSourceProperty")
		purego.RegisterLibFunc(&t.list, carbon, "TISCreateInputSourceList")
		purego.RegisterLibFunc(&t.selectSrc, carbon, "TISSelectInputSource")
		api = t
	})
	return api
}

func (t *tis) id(src uintptr) string {
	s := t.property(src, t.keyID)
	buf := make([]byte, 256)
	if s == 0 || !t.getCString(s, &buf[0], len(buf), cfStringEncodingUTF8) {
		return ""
	}
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

// System switches the real macOS input source.
type System struct{}

func (System) ASCII() string {
	t := load()
	if t == nil {
		return ""
	}
	cur := t.current()
	defer t.release(cur)
	asc := t.ascii()
	defer t.release(asc)
	prev := t.id(cur)
	if prev == t.id(asc) {
		return ""
	}
	t.selectSrc(asc)
	return prev
}

func (System) Select(id string) {
	t := load()
	if t == nil || id == "" {
		return
	}
	l := t.list(0, false)
	defer t.release(l)
	for i := 0; i < t.arrCount(l); i++ {
		if src := t.arrAt(l, i); t.id(src) == id {
			t.selectSrc(src)
			return
		}
	}
}
