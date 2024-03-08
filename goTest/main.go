package main

import (
	"github.com/jopiserver/jsQuickJs"
	"os"
)

func main() {
	jsQuickJs.InitializeEngine()

	filePath := "index.js"
	fileContent, _ := os.ReadFile(filePath)

	ctx := jsQuickJs.CreateContext()
	err := jsQuickJs.ExecuteScript(ctx, string(fileContent), filePath)

	if err != nil {
		err.Print()
	}
}
