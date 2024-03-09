package jsQuickJs

import (
	"fmt"
	"runtime"
)

var gFatalErrorHandler func(err any)

func SafeGoRoutine(f func()) {
	go func() {
		defer CatchFatalErrors()
		f()
	}()
}

func CatchFatalErrors() {
	if err := recover(); err != nil {
		if gFatalErrorHandler != nil {
			gFatalErrorHandler(err)
		} else {
			fmt.Printf("CATCH FATAL ERROR: %s\n", err)
		}
	}
}

func SetFatalErrorHandler(handler func(err any)) {
	gFatalErrorHandler = handler
}

type Disposable interface {
	Dispose()
}

type sOnGcCall struct {
	f func() bool
}

func (m *sOnGcCall) finalizer() {
	if m.f() {
		runOnGcCall(m.f)
	}
}

func runOnGcCall(f func() bool) any {
	e := &sOnGcCall{f}
	runtime.SetFinalizer(e, (*sOnGcCall).finalizer)
	return e
}
