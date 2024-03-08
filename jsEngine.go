package jsQuickJs

// #cgo CFLAGS: -I${SRCDIR}/libQuickJS/src
// #cgo LDFLAGS: -pthread -lquickjs -lm -L${SRCDIR}/libQuickJS/linux_x86_64 -ldl
// #cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/libQuickJS/Darwin
// #cgo linux,amd64 LDFLAGS: -L${SRCDIR}/libQuickJS/linux_x86_64 -ldl
// #cgo linux,arm64 LDFLAGS: -L${SRCDIR}/libQuickJS/linux_arm64 -ldl
//
// #include <stdlib.h> // For C.free
// #include "quickJsEngine.h"
//
import "C"
import "unsafe"

func QuickInitialize() {
	C.quickjs_initialize()
}

func QuickCreateContext() *C.s_quick_ctx {
	return C.quick_createContext()
}

func QuickExecuteScriptString(ctx *C.s_quick_ctx, origin string, script string) C.s_quick_execResult {
	cScript := C.CString(script)
	cOrigin := C.CString(origin)

	defer func() {
		C.free(unsafe.Pointer(cScript))
		C.free(unsafe.Pointer(cOrigin))
	}()

	return C.quickjs_executeScriptString(ctx, cScript, cOrigin)
}
