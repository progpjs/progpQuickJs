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

import (
	"log"
	"reflect"
	"slices"
)

var gFunctionRegistry *FunctionRegistry

//region RegisteredFunction

type RegisteredFunction struct {
	IsAsync         bool
	JsFunctionName  string
	GoFunctionRef   any
	GoFunctionInfos ParsedGoFunction
}

//endregion

//region ParsedGoFunction

type ParsedGoFunction struct {
	GoFunctionName string

	ParamTypes          []string
	ParamTypeRefs       []reflect.Type
	CallParamNamespaces []string

	ReturnType        string
	ReturnErrorOffset int

	JsFunctionName string
	JsGroupName    string
}

func (m *ParsedGoFunction) GetJsFunctionName() string {
	return m.JsFunctionName
}

func (m *ParsedGoFunction) GetGoFunctionName() string {
	return m.GoFunctionName
}

//endregion

//region FunctionRegistry

type FunctionRegistry struct {
	functionsMap map[string]*RegisteredFunction
}

func GetFunctionRegistry() *FunctionRegistry {
	if gFunctionRegistry == nil {
		gFunctionRegistry = &FunctionRegistry{functionsMap: make(map[string]*RegisteredFunction)}
	}

	return gFunctionRegistry
}

func (m *FunctionRegistry) AddFunction(jsName string, goFunction any) {
	entry := &RegisteredFunction{}
	entry.JsFunctionName = jsName
	entry.GoFunctionRef = goFunction

	reflectFct := reflect.TypeOf(goFunction)
	entry.GoFunctionInfos = m.parseGoFunction(jsName, reflectFct)

	m.functionsMap[jsName] = entry
}

//endregion

func (m *FunctionRegistry) parseGoFunction(jsName string, reflectFct reflect.Type) ParsedGoFunction {
	res := ParsedGoFunction{ReturnErrorOffset: -1}

	// > Parse parameters

	inCount := reflectFct.NumIn()
	res.ParamTypeRefs = make([]reflect.Type, inCount)

	for i := 0; i < inCount; i++ {
		param := reflectFct.In(i)
		paramTypeName := param.String()
		res.ParamTypes = append(res.ParamTypes, paramTypeName)
		res.ParamTypeRefs[i] = param

		// > Extract namespace

		// If pointer then take the target type.
		for {
			kind := param.Kind()

			if (kind == reflect.Pointer) || (kind == reflect.Array) || (kind == reflect.Slice) || (kind == reflect.Map) {
				param = param.Elem()
			} else {
				break
			}
		}

		// Add the namespace if enclosed into a namespace.
		//
		pkgPath := param.PkgPath()
		//
		if pkgPath != "" {
			if !slices.Contains(res.CallParamNamespaces, pkgPath) {
				res.CallParamNamespaces = append(res.CallParamNamespaces, pkgPath)
			}
		}
	}

	// > Parse return infos

	outCount := reflectFct.NumOut()
	var returnTypes []string

	for i := 0; i < outCount; i++ {
		outParam := reflectFct.Out(i)
		outType := outParam.String()
		returnTypes = append(returnTypes, outType)
	}

	if outCount >= 1 {
		if outCount == 1 {
			if returnTypes[0] == "error" {
				res.ReturnErrorOffset = 0
				returnTypes = nil
			} else {
				res.ReturnType = returnTypes[0]
			}
		} else if outCount == 2 {
			if returnTypes[0] == "error" {
				res.ReturnErrorOffset = 0
				returnTypes = returnTypes[1:]

				if returnTypes[0] == "error" {
					log.Fatalf("Function %s can return (error, error)", jsName)
				}
			} else if returnTypes[1] == "error" {
				res.ReturnErrorOffset = 1
				returnTypes = returnTypes[0:1]
			} else {
				log.Fatalf("Function %s has more than 1 return type", jsName)
			}

			res.ReturnType = returnTypes[0]
		} else {
			log.Fatalf("Function %s has more than 1 return type", jsName)
		}
	}

	return res
}

type StringBuffer []byte
