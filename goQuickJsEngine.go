/*
 * (C) Copyright 2024 Johan Michel PIQUET, France (https://johanpiquet.fr/).
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package jsQuickJs

import "C"
import (
	"encoding/json"
	"errors"
	"fmt"
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

//region Context

var gScriptOrigin = C.CString("<scriptOrigin>")

var gNextFunctionId = 0
var gIsEngineInitialized bool
var gFunctionTable = make([]registeredFunction, 500)
var gCFunctionTable = make([]*C.JSCFunction, 500)

type Context struct {
	ptr          *C.s_quick_ctx
	isKeepAlive  bool
	taskQueue    *TaskQueue
	scriptOrigin string
	jsError      *JsErrorMessage
	errorHandler ErrorHandler

	refCount      int
	refCountMutex sync.Mutex

	trackedResourceTail *trackedResource
}

type ErrorHandler func(*Context, *JsErrorMessage) bool

func NewContext() *Context {
	if !gIsEngineInitialized {
		panic("quickjs engine isn't initialized")
	}

	DeclareBackgroundTaskStarted()

	m := &Context{}
	m.ptr = C.quickjs_createContext(unsafe.Pointer(m))

	for i := 0; i < gNextFunctionId; i++ {
		jsf := gFunctionTable[i]
		C.quickjs_bindFunction(m.ptr, jsf.name, jsf.paramCount, jsf.cFunction)
	}

	m.taskQueue = newTaskQueue()

	runOnGcCall(func() bool {
		if m.taskQueue.disposed {
			return false
		}

		// Allows to be thread safe without using mutex on add/remove mutex.
		m.taskQueue.Push(func() {
			if m.trackedResourceTail != nil {
				m.cleanUpTrackedResources(false)
			}
		})

		return true
	})

	return m
}

func InitializeEngine() {
	if gIsEngineInitialized {
		return
	}
	gIsEngineInitialized = true

	C.quickjs_cgoInitialize()
	C.quickjs_registerDynamicFunctions()
}

// cleanUpTrackedResources check which track resource can be freed.
// It's called when the GC execute or when the context is destroyed.
// Using this strategy allows
func (m *Context) cleanUpTrackedResources(forceDispose bool) {
	tail := m.trackedResourceTail

	for {
		if tail == nil {
			return
		}

		if tail.isDisposed {
			tail.detach(m)
		} else if forceDispose {
			tail.dispose()
			tail.detach(m)
		}
	}
}

func (m *Context) SetErrorHandler(h ErrorHandler) {
	m.errorHandler = h
}

func (m *Context) Dispose() {
	if m.isKeepAlive {
		// Warning: The C object is never  free du to a possible bug or something I don't understand.
		//			It's why memory is bleeding when creating/disposing a lot of contexts.
		//
		// Here the call is required since it will release some resources.
		// But it don't destroy the context himself.

		C.quickjs_decrContext(m.ptr)
		m.isKeepAlive = false
	}
}

func (m *Context) onCppDisposed() {
	m.taskQueue.Dispose()
	DeclareBackgroundTaskEnded()
}

// KeepAlive allows to avoid destroying the context once the script executed.
// Without that the internal ref counter will automatically destroy the context.
func (m *Context) KeepAlive() {
	if m.isKeepAlive {
		return
	}

	m.isKeepAlive = true
	C.quickjs_incrContext(m.ptr)

	// Will dispose the context on the GC run if no more references.
	runtime.SetFinalizer(m, (*Context).Dispose)
}

func (m *Context) incrRef() {
	m.refCountMutex.Lock()

	if m.refCount == 0 {
		C.quickjs_incrContext(m.ptr)
	}

	m.refCount++
	m.refCountMutex.Unlock()
}

func (m *Context) decrRef() {
	m.refCountMutex.Lock()

	m.refCount--
	if m.refCount == 0 {
		m.dispose()
	}

	m.refCountMutex.Unlock()
}

func (m *Context) dispose() {
	// Will destroy the context and the runtime when gc-ref is 0.
	// We context keep the context since his memory isn't really free until the runtime is free.

	C.quickjs_decrContext(m.ptr)
	m.cleanUpTrackedResources(true)
	m.trackedResourceTail = nil
}

func (m *Context) ExecuteScript(script string, scriptOrigin string) *JsErrorMessage {
	cScript := C.CString(script)
	defer func() { C.free(unsafe.Pointer(cScript)) }()

	m.scriptOrigin = scriptOrigin

	jsErr := C.quickjs_executeScript(m.ptr, cScript, gScriptOrigin)
	if jsErr != nil {
		m.processError(jsErr)
	}

	return m.jsError
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

		// Occurs when saturating.
		//
		if idx2 != -1 {
			frame.Function = line[idx1+3 : idx2-1]
			frame.Source = scriptPath

			idx1 = strings.LastIndex(line, ":")
			idx2 = strings.LastIndex(line, ")")
			line := line[idx1+1 : idx2]
			frame.Line, _ = strconv.Atoi(line)

			res.StackTraceFrames = append(res.StackTraceFrames, frame)
		}
	}

	return res
}

func (m *Context) processError(jsErr *C.s_quick_error) {
	m.jsError = m.createErrorMessage(m.scriptOrigin, C.GoString(jsErr.errorTitle), C.GoString(jsErr.errorStackTrace))
	defer C.quickjs_releaseError(jsErr)

	if m.errorHandler != nil {
		canContinue := m.errorHandler(m, m.jsError)
		if canContinue {
			return
		}
	} else {
		m.jsError.Print()
	}

	m.taskQueue.Dispose()
}

// trackResource will add a resource to the tracker.
// Warning: must be called from the task queue in order to be thread safe.
func (m *Context) trackResource(res any) C.uintptr_t {
	tr := &trackedResource{value: res}
	tr.before = m.trackedResourceTail
	if tr.before != nil {
		tr.before.next = tr
	}

	m.trackedResourceTail = tr
	return (C.uintptr_t)(uintptr(unsafe.Pointer(tr)))
}

//endregion

//region JsFunction

type JsFunction struct {
	ptr               *C.JSValue
	ctx               *Context
	comeFromAsyncCall bool
	keepAlive         C.int
}

func (m *JsFunction) Dispose() {
	if m.ptr != nil {
		C.quickjs_releaseFunction(m.ctx.ptr, (*C.JSValue)(m.ptr))
		m.ptr = nil
	}
}

func (m *JsFunction) CallWithUndefined() {
	if m.ptr == nil {
		return
	}

	m.ctx.taskQueue.Push(func() {
		err := C.quickjs_callFunctionWithUndefined(m.ctx.ptr, m.ptr, m.keepAlive)
		m.ptr = nil

		if m.comeFromAsyncCall {
			m.comeFromAsyncCall = false
			m.ctx.decrRef()
		}

		if err != nil {
			m.ctx.processError(err)
		}
	})
}

func (m *JsFunction) CallWithAutoReleaseResource2(res any) {
	if m.ptr == nil {
		return
	}

	m.ctx.taskQueue.Push(func() {
		trResource := m.ctx.trackResource(res)
		err := C.quickjs_callFunctionWithAutoReleaseResource2(m.ctx.ptr, m.ptr, m.keepAlive, trResource)
		m.ptr = nil

		if m.comeFromAsyncCall {
			m.comeFromAsyncCall = false
			m.ctx.decrRef()
		}

		if err != nil {
			m.ctx.processError(err)
		}
	})
}

func (m *JsFunction) CallWithAsync(values ...any) {
	if m.ptr == nil {
		return
	}

	m.ctx.taskQueue.Push(func() {
		m.doCallWith(values, false)
	})
}

func (m *JsFunction) CallWith(values ...any) AnyValue {
	if m.ptr == nil {
		return AnyValue{Type: cAnyValueTypeUndefined}
	}

	return m.doCallWith(values, true)
}

func (m *JsFunction) doCallWith(values []any, sendErrorToJavascript bool) AnyValue {
	m.ctx.ptr.goToJsValuesCount = C.int(len(values))
	cOutputParams := m.ctx.ptr.goToJsValues

	for _, v := range values {
		goValueToCAnyValue(v, cOutputParams)
		cOutputParams = (*C.s_quick_anyValue)(unsafe.Add(unsafe.Pointer(cOutputParams), C.sizeof_s_quick_anyValue))
	}

	res := C.quickjs_callFunctionWithAnyValues(m.ctx.ptr, m.ptr, m.keepAlive)
	runtime.KeepAlive(cOutputParams)

	if m.comeFromAsyncCall {
		m.comeFromAsyncCall = false

		// Must be keep in the defer.
		// Without that the call to cAnyValueToGoAnyValue will crash.
		//
		defer func() {
			m.ctx.decrRef()
		}()
	}

	if res.error != nil {
		if sendErrorToJavascript {
			// Here the error can be try/catch from javascript.
			// But we can't have the stacktrace, only the title.
			//
			return AnyValue{Type: cAnyValueTypeError, Value: errors.New(C.GoString(res.error.errorTitle))}
		} else {
			// Here Go process the error and print the stacktrace.
			// It's must be used if the function isn't intended to be call from javascript directly.
			m.ctx.processError(res.error)
		}
	}

	return cAnyValueToGoAnyValue(m.ctx, false, &res.returnValue)
}

func (m *JsFunction) KeepAlive() {
	if m.ptr != nil {
		if m.keepAlive != cInt1 {
			m.keepAlive = cInt1
			runtime.SetFinalizer(m, (*JsFunction).Dispose)
		}
	}
}

//endregion

//region Tracked Resources

type trackedResource struct {
	value      any
	isDisposed bool
	before     *trackedResource
	next       *trackedResource
	isDetached bool
}

func (m *trackedResource) detach(ctx *Context) {
	if m.isDetached {
		return
	}
	m.isDetached = true

	if m.next != nil {
		m.next.before = m.before
	} else if ctx.trackedResourceTail == m {
		ctx.trackedResourceTail = m.before
	}

	if m.before != nil {
		m.before.next = m.next
	}
}

func (m *trackedResource) dispose() {
	m.isDisposed = true

	if d, ok := m.value.(Disposable); ok {
		d.Dispose()
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

//region Function registration

type registeredFunction struct {
	name       *C.char
	goFunction func(call *JsToGoCall)

	paramCount C.int
	cFunction  *C.JSCFunction
	isAsync    bool
}

func RegisterAsyncFunction(jsName string, toCall func(call *JsToGoCall)) {
	auxRegisterFunction(jsName, toCall, true)
}

func RegisterFunction(jsName string, toCall func(call *JsToGoCall)) {
	auxRegisterFunction(jsName, toCall, false)
}

func auxRegisterFunction(jsName string, toCall func(call *JsToGoCall), isAsync bool) {
	id := gNextFunctionId
	gNextFunctionId++

	gFunctionTable[id] = registeredFunction{
		name:       C.CString(jsName),
		goFunction: toCall,
		cFunction:  gCFunctionTable[id],
		isAsync:    isAsync,
	}
}

//endregion

//region AnyValue

type AnyValueType int

var cAnyValueTypeUndefined = C.int(0)
var cAnyValueTypeNull = C.int(1)
var cAnyValueTypeError = C.int(2)
var cAnyValueTypeNumber = C.int(3)
var cAnyValueTypeString = C.int(4)
var cAnyValueTypeBoolean = C.int(5)
var cAnyValueTypeBuffer = C.int(6)
var cAnyValueTypeFunction = C.int(7)
var cAnyValueTypeJson = C.int(8)
var cAnyValueTypeInt32 = C.int(9)

var cInt1 = C.int(1)
var cInt0 = C.int(0)

func cAnyValueToGoAnyValue(ctx *Context, isAsync bool, cAnyValue *C.s_quick_anyValue) AnyValue {
	cValueType := cAnyValue.valueType

	if cValueType == cAnyValueTypeInt32 {
		return AnyValue{Type: cValueType, Value: int(cAnyValue.size)}
	} else if cValueType == cAnyValueTypeNumber {
		return AnyValue{Type: cValueType, Value: float64(cAnyValue.number)}
	} else if cValueType == cAnyValueTypeBoolean {
		return AnyValue{Type: cValueType, Value: cAnyValue.size == cInt1}
	} else if cValueType == cAnyValueTypeString {
		return AnyValue{Type: cValueType, Value: C.GoString((*C.char)(cAnyValue.voidPtr))}
	} else if cValueType == cAnyValueTypeJson {
		return AnyValue{Type: cValueType, Value: C.GoString((*C.char)(cAnyValue.voidPtr))}
	} else if cValueType == cAnyValueTypeBuffer {
		return AnyValue{Type: cValueType, Value: C.GoBytes(cAnyValue.voidPtr, (C.int)(cAnyValue.size))}
	} else if cValueType == cAnyValueTypeFunction {
		return AnyValue{Type: cValueType, Value: &JsFunction{ctx: ctx, comeFromAsyncCall: isAsync, ptr: (*C.JSValue)(cAnyValue.voidPtr)}}
	}

	return AnyValue{Type: cAnyValueTypeUndefined}
}

func goValueToCAnyValue(goVal any, cAnyVal *C.s_quick_anyValue) {
	cAnyVal.mustFree = cInt0

	if goVal == nil {
		cAnyVal.valueType = cAnyValueTypeUndefined
		return
	}

	if asInt, ok := goVal.(int); ok {
		cAnyVal.valueType = cAnyValueTypeNumber
		cAnyVal.number = C.double(float64(asInt))
		return
	}

	if asFloat, ok := goVal.(float64); ok {
		cAnyVal.valueType = cAnyValueTypeNumber
		cAnyVal.number = C.double(asFloat)
		return
	}

	if asBool, ok := goVal.(bool); ok {
		cAnyVal.valueType = cAnyValueTypeBoolean

		if asBool {
			cAnyVal.size = cInt1
		} else {
			cAnyVal.size = cInt0
		}

		return
	}

	if asString, ok := goVal.(string); ok {
		cAnyVal.valueType = cAnyValueTypeString
		cAnyVal.voidPtr = unsafe.Pointer(C.CString(asString))
		cAnyVal.size = C.int(len(asString))
		cAnyVal.mustFree = cInt1
		return
	}

	if asBuffer, ok := goVal.([]byte); ok {
		cAnyVal.valueType = cAnyValueTypeBuffer

		// Must copy the buffer here in order to avoid using the Go one.
		cAnyVal.voidPtr = C.quickjs_copyBuffer(unsafe.Pointer(&asBuffer[0]), C.int(len(asBuffer)))
		cAnyVal.size = C.int(len(asBuffer))
		cAnyVal.mustFree = cInt1

		return
	}

	if asError, ok := goVal.(error); ok {
		asString := asError.Error()
		cAnyVal.valueType = cAnyValueTypeError
		cAnyVal.voidPtr = unsafe.Pointer(C.CString(asString))
		cAnyVal.size = C.int(len(asString))
		cAnyVal.mustFree = cInt1
		return
	}

	bAsJson, err := json.Marshal(goVal)

	if err == nil {
		cAnyVal.valueType = cAnyValueTypeJson
		cAnyVal.voidPtr = unsafe.Pointer(&bAsJson[0])
		cAnyVal.size = C.int(len(bAsJson))
		return
	}

	// > Error

	asString := string(bAsJson)
	cAnyVal.valueType = cAnyValueTypeError
	cAnyVal.voidPtr = unsafe.Pointer(C.CString(asString))
	cAnyVal.size = C.int(len(asString))
	cAnyVal.mustFree = cInt1
}

type AnyValue struct {
	Type  C.int
	Value any
}

func (m AnyValue) AssertIsFunction() bool {
	_, ok := m.Value.(*JsFunction)
	return ok
}

func (m AnyValue) AssertIsInt() bool {
	_, ok := m.Value.(int)
	return ok
}

func (m AnyValue) AssertIsFloat64() bool {
	_, ok := m.Value.(float64)
	return ok
}

func (m AnyValue) AssertIsBool() bool {
	_, ok := m.Value.(bool)
	return ok
}

func (m AnyValue) AssertIsByte() bool {
	_, ok := m.Value.([]byte)
	return ok
}

func (m AnyValue) AsFunction() *JsFunction {
	f, ok := m.Value.(*JsFunction)
	if ok {
		return f
	}

	return nil
}

func (m AnyValue) AsInt() int {
	return m.Value.(int)
}

func (m AnyValue) AsFloat64() float64 {
	return m.Value.(float64)
}

func (m AnyValue) AsBool() bool {
	return m.Value.(bool)
}

func (m AnyValue) AsString() string {
	return m.Value.(string)
}

func (m AnyValue) AsBuffer() []byte {
	return m.Value.([]byte)
}

func (m AnyValue) AsAny() any {
	if m.Type == cAnyValueTypeJson {
		if s, ok := m.Value.(string); ok {
			var myMap map[string]any
			_ = json.Unmarshal([]byte(s), &myMap)
			m.Value = myMap
		}
	}

	return m.Value
}

//endregion

//region JsToGoCall

type JsToGoCall struct {
	Args        []AnyValue
	ctx         *Context
	returnValue any
}

func (m *JsToGoCall) AssertArgCount(count int) bool {
	if len(m.Args) < count {
		m.returnValue = errors.New(fmt.Sprintf("call param error: %d argument expected", count))
		return false
	}

	return true
}

func (m *JsToGoCall) SetReturnValue(value any) {
	if av, ok := value.(AnyValue); ok {
		value = av.AsAny()
	}

	m.returnValue = value
}

//endregion

//region Call from C++

//export cgoRegisterDynamicFunction
func cgoRegisterDynamicFunction(id C.int, fctRef *C.JSCFunction) {
	gCFunctionTable[int(id)] = fctRef
}

//export cgoOnContextReleased
func cgoOnContextReleased(pCtx *C.s_quick_ctx) {
	goCtx := (*Context)(unsafe.Pointer(pCtx.userData))
	goCtx.onCppDisposed()
}

//export cgoOnAutoDisposeResourceReleased
func cgoOnAutoDisposeResourceReleased(res unsafe.Pointer) {
	(*trackedResource)(res).dispose()
}

//export cgoCallDynamicFunction
func cgoCallDynamicFunction(functionId C.int, pCtx *C.s_quick_ctx, argc C.int) C.s_quick_anyValue {
	jsToGoValues := pCtx.jsToGoValues
	goCtx := (*Context)(unsafe.Pointer(pCtx.userData))

	count := int(argc)
	jsf := gFunctionTable[int(functionId)]

	cAnyValue := jsToGoValues
	var allAnyValues []AnyValue

	for i := 0; i < count; i++ {
		allAnyValues = append(allAnyValues, cAnyValueToGoAnyValue(goCtx, jsf.isAsync, cAnyValue))
		cAnyValue = (*C.s_quick_anyValue)(unsafe.Add(unsafe.Pointer(cAnyValue), C.sizeof_s_quick_anyValue))
	}

	if jsf.isAsync {
		goCtx.incrRef()
	}

	call := JsToGoCall{ctx: goCtx, Args: allAnyValues}
	jsf.goFunction(&call)

	var cResult C.s_quick_anyValue
	goValueToCAnyValue(call.returnValue, &cResult)

	return cResult
}

//endregion
