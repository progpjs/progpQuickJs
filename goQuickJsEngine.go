package jsQuickJs

import "C"
import (
	"runtime"
	"strconv"
	"strings"
	"sync"
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
// #include "cgo.h"
//
import "C"

var cUInt8True = C.uint8_t(1)
var gScriptOrigin = C.CString("<scriptOrigin>")

var gNextFunctionId = 0
var gFunctionTable = make([]registeredFunction, 500)
var gCFunctionTable = make([]*C.JSCFunction, 500)

type registeredFunction struct {
	name       *C.char
	goFunction func(ctx *Context, value []AnyValue)

	paramCount C.int
	cFunction  *C.JSCFunction
	isAsync    bool
}

func InitializeEngine() {
	C.quickjs_cgoInitialize()
	C.quickjs_registerDynamicFunctions()
}

func RegisterAsyncFunction(jsName string, toCall func(ctx *Context, value []AnyValue)) {
	auxRegisterFunction(jsName, toCall, true)
}

func RegisterFunction(jsName string, toCall func(ctx *Context, value []AnyValue)) {
	auxRegisterFunction(jsName, toCall, false)
}

func auxRegisterFunction(jsName string, toCall func(ctx *Context, value []AnyValue), isAsync bool) {
	id := gNextFunctionId
	gNextFunctionId++

	gFunctionTable[id] = registeredFunction{
		name:       C.CString(jsName),
		goFunction: toCall,
		cFunction:  gCFunctionTable[id],
		isAsync:    isAsync,
	}
}

//region Context

type Context struct {
	ctx         *C.s_quick_ctx
	isKeepAlive bool

	refCount      int
	refCountMutex sync.Mutex
}

func NewContext() *Context {
	DeclareBackgroundTaskStarted()

	m := &Context{}
	m.ctx = C.quick_createContext(unsafe.Pointer(m))

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

func (m *Context) onCppDisposed() {
	DeclareBackgroundTaskEnded()
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

func (m *Context) incrRef() {
	m.refCountMutex.Lock()

	if m.refCount == 0 {
		C.quickjs_incrContext(m.ctx)
	}

	m.refCount++
	m.refCountMutex.Unlock()
}

func (m *Context) decrRef() {
	m.refCountMutex.Lock()

	m.refCount++
	if m.refCount == 0 {
		C.quickjs_decrContext(m.ctx)
	}

	m.refCountMutex.Unlock()
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

func (m *Context) CallFunction(fctRef unsafe.Pointer) {
	C.quickjs_callFunction(m.ctx, (*C.JSValue)(fctRef))
}

//endregion

//region JsFunction

type JsFunction struct {
	ptr     unsafe.Pointer
	ctx     *Context
	isAsync bool
}

func (m *JsFunction) CallWithUndefined() {
	m.ctx.CallFunction(m.ptr)

	if m.isAsync {
		m.ctx.decrRef()
	}
}

//endregion

//region Background tasks

var gBackgroundTasksCount = 0
var gBackgroundTasksCountMutex sync.Mutex
var gBackgroundTasksWaitChannel = make(chan bool)

func DeclareBackgroundTaskStarted() {
	gBackgroundTasksCountMutex.Lock()
	gBackgroundTasksCount++
	gBackgroundTasksCountMutex.Unlock()
}

func DeclareBackgroundTaskEnded() {
	gBackgroundTasksCountMutex.Lock()
	defer gBackgroundTasksCountMutex.Unlock()

	if gBackgroundTasksCount != 0 {
		gBackgroundTasksCount--
	}

	if gBackgroundTasksCount == 0 {
		close(gBackgroundTasksWaitChannel)
		gBackgroundTasksWaitChannel = nil
	}
}

// WaitTasksEnd wait until all background tasks are finished.
// It's used in order to know if the application can exit.
func WaitTasksEnd() {
	if gBackgroundTasksWaitChannel != nil {
		<-gBackgroundTasksWaitChannel
	}
}

//endregion

//region AnyValue

type AnyValueType int

var cAnyValueTypeUndefined = C.int(0)
var cAnyValueTypeNull = C.int(1)
var cAnyValueTypeInvalid = C.int(2)
var cAnyValueTypeNumber = C.int(3)
var cAnyValueTypeString = C.int(4)
var cAnyValueTypeBoolean = C.int(5)
var cAnyValueTypeBuffer = C.int(6)
var cAnyValueTypeFunction = C.int(7)
var cAnyValueTypeJson = C.int(8)
var cAnyValueTypeInt32 = C.int(9)

type AnyValue struct {
	Type  C.int
	Value any
}

var cInt1 = C.int(1)

//endregion

//region Call from C++

//export cgoRegisterDynamicFunction
func cgoRegisterDynamicFunction(id C.int, fctRef *C.JSCFunction) {
	gCFunctionTable[int(id)] = fctRef
}

//export cgoCallDynamicFunction
func cgoCallDynamicFunction(functionId C.int, pCtx *C.s_quick_ctx, argc C.int) C.JSValue {
	inputAnyValues := pCtx.inputAnyValues
	goCtx := (*Context)(unsafe.Pointer(pCtx.userData))

	count := int(argc)
	jsf := gFunctionTable[int(functionId)]

	cAnyValue := inputAnyValues
	var allAnyValues []AnyValue

	for i := 0; i < count; i++ {
		cValueType := cAnyValue.valueType

		if cValueType == cAnyValueTypeInt32 {
			v := AnyValue{Type: cValueType, Value: int(cAnyValue.size)}
			allAnyValues = append(allAnyValues, v)
		} else if cValueType == cAnyValueTypeNumber {
			v := AnyValue{Type: cValueType, Value: float64(cAnyValue.number)}
			allAnyValues = append(allAnyValues, v)
		} else if cValueType == cAnyValueTypeBoolean {
			v := AnyValue{Type: cValueType, Value: cAnyValue.size == cInt1}
			allAnyValues = append(allAnyValues, v)
		} else if cValueType == cAnyValueTypeString {
			v := AnyValue{Type: cValueType, Value: C.GoString((*C.char)(cAnyValue.voidPtr))}
			allAnyValues = append(allAnyValues, v)
		} else if cValueType == cAnyValueTypeJson {
			v := AnyValue{Type: cValueType, Value: C.GoString((*C.char)(cAnyValue.voidPtr))}
			allAnyValues = append(allAnyValues, v)
		} else if cValueType == cAnyValueTypeBuffer {
			v := AnyValue{Type: cValueType, Value: C.GoBytes(cAnyValue.voidPtr, (C.int)(cAnyValue.size))}
			allAnyValues = append(allAnyValues, v)
		} else if cValueType == cAnyValueTypeFunction {
			v := AnyValue{Type: cValueType, Value: &JsFunction{ctx: goCtx, isAsync: jsf.isAsync, ptr: unsafe.Pointer(cAnyValue.voidPtr)}}
			allAnyValues = append(allAnyValues, v)
		}

		cAnyValue = (*C.s_progp_anyValue)(unsafe.Add(unsafe.Pointer(cAnyValue), C.sizeof_s_progp_anyValue))
	}

	if jsf.isAsync {
		goCtx.incrRef()
	}

	jsf.goFunction(goCtx, allAnyValues)
	return C.JS_UNDEFINED
}

//export cgoOnContextDestroyed
func cgoOnContextDestroyed(pCtx *C.s_quick_ctx) {
	goCtx := (*Context)(unsafe.Pointer(pCtx.userData))
	goCtx.onCppDisposed()
}

//endregion
