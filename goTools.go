package jsQuickJs

import "fmt"

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
