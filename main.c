#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "quickJsEngine.h"

#ifdef QUICKJSENGINE_STANDALONE

//region Sample functions

static void quickFreePointer(JSRuntime *rt, void *opaque, void *ptr) {
    free(ptr);
}

static JSValue js_print(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv)
{
    int i;
    const char *str;

    for (i = 0; i < argc; i++) {
        str = JS_ToCString(ctx, argv[i]);
        if (!str) return JS_EXCEPTION;
        DEBUG_PRINT(str);
        JS_FreeCString(ctx, str);
    }

    return JS_UNDEFINED;
}

static JSValue js_stringToArrayBuffer(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
    const char* str = JS_ToCString(ctx, argv[0]);
    if (!str) return JS_EXCEPTION;

    char* buffer = strdup(str);
    JS_FreeCString(ctx, str);

    return JS_NewArrayBuffer(ctx, (uint8_t*)buffer, strlen(buffer), quickFreePointer, NULL, false);
}

static JSValue js_arrayBufferToString(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
    JSValue arg0 = argv[0];
    size_t size;
    uint8_t* buffer = JS_GetArrayBuffer(ctx, &size, arg0);
    return JS_NewString(ctx, (const char*) buffer);
}

static JSValue js_receiveFunction(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
    JSValue arg0 = argv[0];

    if (JS_IsFunction(ctx, arg0)) {
        return JS_Call(ctx, arg0, JS_UNDEFINED, 0, NULL);
    }

    return JS_UNDEFINED;
}

static JSValue js_test(JSContext *ctx, JSValueConst this_val, int argc, JSValueConst *argv) {
    const char * str = strdup("mon errreur");

    s_quick_ctx* pCtx = quickjs_callParamsToAnyValue(ctx, argc, argv);

    s_quick_anyValue err;
    err.valueType = AnyValueTypeError;
    err.voidPtr = (void*)str;
    err.size = strlen(str);
    err.mustFree = true;

    return quickjs_processExternalFunctionCallResult(pCtx, err);
}

//endregion

char* readFile(const char* filePath) {
    char *source = NULL;
    FILE *fp = fopen(filePath, "r");

    if (fp != NULL) {
        if (fseek(fp, 0L, SEEK_END) == 0) {
            long bufsize = ftell(fp);
            if (bufsize == -1) { fclose(fp); return NULL; }

            source = malloc(sizeof(char) * (bufsize + 1));

            if (fseek(fp, 0L, SEEK_SET) != 0) { fclose(fp); return NULL; }

            size_t newLen = fread(source, sizeof(char), bufsize, fp);

            if (ferror(fp) != 0) {
                fclose(fp); return NULL;
            } else {
                source[newLen++] = '\0';
            }
        }

        fclose(fp);
    }

    return source;
}

int main(int argc, char **argv)
{
    quickjs_initialize();

    const char* scriptPath = "index.js";
    char* scriptContent = readFile("index.js");
    if (scriptContent==NULL) return 1;

    s_quick_ctx* pCtx = quickjs_createContext(NULL);
    quickjs_bindFunction(pCtx, "js_print", 1, js_print);
    quickjs_bindFunction(pCtx, "js_stringToArrayBuffer", 1, js_stringToArrayBuffer);
    quickjs_bindFunction(pCtx, "js_arrayBufferToString", 1, js_arrayBufferToString);
    quickjs_bindFunction(pCtx, "js_receiveFunction", 1, js_receiveFunction);
    quickjs_bindFunction(pCtx, "js_test", 1, js_test);

    s_quick_error* err = quickjs_executeScript(pCtx, scriptContent, scriptPath);

    if (err!=NULL) {
        DEBUG_PRINT("ERROR: ");
        DEBUG_PRINT_KeepLine(err->errorTitle);
        DEBUG_PRINT_KeepLine(err->errorStackTrace);
    }

    free(scriptContent);
    return 0;
}

#endif // QUICKJSENGINE_STANDALONE
