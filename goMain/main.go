package main

import "github.com/jopiserver/jsQuickJs"

func main() {
	jsQuickJs.QuickInitialize()

	ctx := jsQuickJs.QuickCreateContext()

	jsQuickJs.QuickExecuteScriptString(ctx, "<noscript>", `
		console.log("hello");
	`)
}
