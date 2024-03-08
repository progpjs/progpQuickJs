package jsQuickJs

import "C"
import (
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"unsafe"
)

// #cgo CFLAGS: -I${SRCDIR}/libQuickJS/src
// #cgo LDFLAGS: -pthread -lquickjs
// #cgo darwin,arm64 LDFLAGS: -L${SRCDIR}/libQuickJS/Darwin
// #cgo linux,amd64 LDFLAGS: -L${SRCDIR}/libQuickJS/linux_x86_64 -ldl
// #cgo linux,arm64 LDFLAGS: -L${SRCDIR}/libQuickJS/linux_arm64 -ldl
//
// #include <stdlib.h> // For C.free
// #include "quickJsEngine.h"
// #include "cgoBinding.h"
// #include "dynamic.h"
//
import "C"

var cUInt8True = C.uint8_t(1)
var gScriptOrigin = C.CString("<scriptOrigin>")

func InitializeEngine() {
	C.quickjs_initialize()
	C.cgoInitialize()
	C.cgo_registerDynamicFunctions()
}

var gNextFunctionId = 0
var gFunctionTable = make([]jsFunction, 500)
var gCFunctionTable = make([]*C.JSCFunction, 500)

type jsFunction struct {
	name       *C.char
	goFunction func()
	paramCount C.int
	cFunction  *C.JSCFunction
}

func RegisterFunction(jsName string, toCall func()) {
	id := gNextFunctionId
	gNextFunctionId++

	gFunctionTable[id] = jsFunction{
		name:       C.CString(jsName),
		goFunction: toCall,
		cFunction:  gCFunctionTable[id],
	}
}

//region Context

type Context struct {
	ctx         *C.s_quick_ctx
	isKeepAlive bool
}

func NewContext() *Context {
	m := &Context{ctx: C.quick_createContext()}

	for i := 0; i < gNextFunctionId; i++ {
		jsf := gFunctionTable[i]
		C.quickjs_bindFunction(m.ctx, jsf.name, jsf.paramCount, jsf.cFunction)
	}

	return m
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
		return m.createErrorMessage(scriptOrigin, C.GoString(res.errorTitle), C.GoString(res.errorStackTrace))
	}

	return nil
}

func (m *Context) createErrorMessage(scriptPath string, title string, body string) *JsErrorMessage {
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

//endregion

//region AnyValue

type AnyValueType int

const (
	AnyValueTypeUndefined AnyValueType = iota
	AnyValueTypeNull
	AnyValueTypeInvalid
	AnyValueTypeNumber
	AnyValueTypeString
	AnyValueTypeBoolean
	AnyValueTypeBuffer
	AnyValueTypeFunction
	AnyValueTypeJson
)

const cInt0 = C.int(0)
const cInt1 = C.int(1)

//region AnyValue from reflect.Value

func AnyValueFromReflectValueString(resV reflect.Value, anyValue *C.s_progp_anyValue) {
	anyValue.valueType = C.int(AnyValueTypeString)
	anyValue.voidPtr = unsafe.Pointer(C.CString(resV.String()))
	anyValue.mustFree = cInt1
}

func AnyValueFromReflectValueBool(resV reflect.Value, anyValue *C.s_progp_anyValue) {
	anyValue.valueType = C.int(AnyValueTypeBoolean)

	if resV.Bool() == true {
		anyValue.size = cInt1
	} else {
		anyValue.size = cInt0
	}
}

func AnyValueFromReflectValueFloat64(resV reflect.Value, anyValue *C.s_progp_anyValue) {
	anyValue.valueType = C.int(AnyValueTypeNumber)
	anyValue.number = C.double(resV.Float())
}

//endregion

func createErrorAnyValue(error string) C.s_progp_anyValue {
	var ev C.s_progp_anyValue
	ev.errorMessage = C.CString(error)
	return ev
}

//endregion

//region Call from C++

//export registerDynamicFunction
func registerDynamicFunction(id C.int, fctRef *C.JSCFunction) {
	gCFunctionTable[int(id)] = fctRef
}

//export goCallDynamic
func goCallDynamic(id C.int, ctx *C.JSContext, argc C.int, argv *C.JSValueConst) C.JSValue {
	jsf := gFunctionTable[int(id)]
	jsf.goFunction()
	return C.JS_UNDEFINED
}

//endregion
