package main

import (
	"github.com/jopiserver/jsQuickJs"
	"os"
)

func main() {
	jsQuickJs.QuickInitialize()

	filePath := "index.js"
	fileContent, _ := os.ReadFile(filePath)

	ctx := jsQuickJs.QuickCreateContext()
	err := jsQuickJs.QuickExecuteScriptString(ctx, string(fileContent), filePath)

	if err != nil {
		err.Print()
	}
}
