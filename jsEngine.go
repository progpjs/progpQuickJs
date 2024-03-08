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
	"runtime"
	"strconv"
	"strings"
	"unsafe"
)

var cUInt8True = C.uint8_t(1)
var gScriptOrigin = C.CString("<scriptOrigin>")

func InitializeEngine() {
	C.quickjs_initialize()
}

type Context struct {
	ctx         *C.s_quick_ctx
	isKeepAlive bool
}

func NewContext() *Context {
	return &Context{ctx: C.quick_createContext()}
}

func (m *Context) Dispose() {
	if m.isKeepAlive {
		C.quickjs_decrContext(m.ctx)
		m.isKeepAlive = false
	}
}

// KeepAlive allows to avoid destroying the context once the script executed.
// Without that the internal ref counter will automatically destroy the context.
func (m *Context) KeepAlive() {
	if m.isKeepAlive {
		return
	}

	m.isKeepAlive = true
	C.quickjs_incrContext(m.ctx)

	// Will dispose the context on the GC run if no more references.
	runtime.SetFinalizer(m, (*Context).Dispose)
}

func (m *Context) ExecuteScript(script string, scriptOrigin string) *JsErrorMessage {
	cScript := C.CString(script)
	defer func() { C.free(unsafe.Pointer(cScript)) }()

	res := C.quickjs_executeScript(m.ctx, cScript, gScriptOrigin)

	if res.isException == cUInt8True {
		return createErrorMessage(scriptOrigin, C.GoString(res.errorTitle), C.GoString(res.errorStackTrace))
	}

	return nil
}

func createErrorMessage(scriptPath string, title string, body string) *JsErrorMessage {
	res := &JsErrorMessage{}
	res.Error = title
	res.ScriptPath = scriptPath

	lines := strings.Split(body, "\n")

	for _, line := range lines {
		frame := StackTraceFrame{}
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
