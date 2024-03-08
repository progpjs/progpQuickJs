package jsQuickJs

// #cgo CFLAGS: -I${SRCDIR}/libQuickJS/src
// #cgo LDFLAGS: -pthread -lquickjs
// #cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/libQuickJS/Darwin
// #cgo linux,amd64 LDFLAGS: -L${SRCDIR}/libQuickJS/linux_x86_64 -ldl
// #cgo linux,arm64 LDFLAGS: -L${SRCDIR}/libQuickJS/linux_arm64 -ldl
//
// #include <stdlib.h> // For C.free
// #include "quickJsEngine.h"
//
import "C"
import (
	"github.com/jopiserver/jsCommon"
	"strconv"
	"strings"
	"unsafe"
)

var cUInt8True = C.uint8_t(1)

func QuickInitialize() {
	C.quickjs_initialize()
}

func QuickCreateContext() *C.s_quick_ctx {
	return C.quick_createContext()
}

var gScriptOrigin = C.CString("<scriptOrigin>")

func QuickExecuteScriptString(ctx *C.s_quick_ctx, script string, scriptOrigin string) *jsCommon.JsErrorMessage {
	cScript := C.CString(script)
	defer func() { C.free(unsafe.Pointer(cScript)) }()

	res := C.quickjs_executeScriptString(ctx, cScript, gScriptOrigin)

	if res.isException == cUInt8True {
		return createErrorMessage(scriptOrigin, C.GoString(res.errorTitle), C.GoString(res.errorStackTrace))
	}

	return nil
}

func createErrorMessage(scriptPath string, title string, body string) *jsCommon.JsErrorMessage {
	res := &jsCommon.JsErrorMessage{}
	res.Error = title
	res.ScriptPath = scriptPath

	lines := strings.Split(body, "\n")

	for _, line := range lines {
		frame := jsCommon.StackTraceFrame{}
		idx1 := strings.Index(line, "at")
		if idx1 == -1 {
			break
		}

		idx2 := strings.Index(line, "(")
		frame.Function = line[idx1+3 : idx2-1]
		frame.Source = scriptPath

		idx1 = strings.LastIndex(line, ":")
		idx2 = strings.LastIndex(line, ")")
		line := line[idx1+1 : idx2]
		frame.Line, _ = strconv.Atoi(line)

		res.StackTraceFrames = append(res.StackTraceFrames, frame)
	}

	return res
}
