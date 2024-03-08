package jsQuickJs

import "fmt"

var gErrorTranslator func(err *JsErrorMessage)

func SetErrorTranslator(h func(err *JsErrorMessage)) {
	gErrorTranslator = h
}

type JsErrorMessage struct {
	isTranslated bool

	Error string

	ScriptPath   string
	SourceMapUrl string
	SourceMap    string

	StackTraceFrames []StackTraceFrame
}

type StackTraceFrame struct {
	Line     int
	Column   int
	Function string
	Source   string
}

func (m *JsErrorMessage) Translate() {
	if m.isTranslated {
		return
	}
	m.isTranslated = true

	if gErrorTranslator != nil {
		gErrorTranslator(m)
	}
}

func (m *JsErrorMessage) StackTrace() string {
	m.Translate()
	res := ""

	for _, frame := range m.StackTraceFrames {
		if frame.Function == "" {
			res += fmt.Sprintf("at %s:%d:%d\n", frame.Source, frame.Line, frame.Column)
		} else {
			res += fmt.Sprintf("at %s:%d:%d: %s\n", frame.Source, frame.Line, frame.Column, frame.Function)
		}
	}

	return res
}

func (m *JsErrorMessage) Print() {
	m.Translate()
	fmt.Printf("Javascript Error - %s\n", m.Error)
	print(m.StackTrace())
}
