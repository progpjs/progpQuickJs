#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>
#include <math.h>
#include "quickJsEngine.h"

#ifdef PROGP_STANDALONE

// To see:
// ctx.JS_GetException
// JS_EXCEPTION
// JS_IsException
// Async: https://www.freelists.org/post/quickjs-devel/Unable-to-await-promise,1

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
        PROGP_PRINT(str);
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

JSValue jobA(JSContext *ctx, int argc, JSValueConst *argv) {
    return JS_UNDEFINED;
}

int main(int argc, char **argv)
{
    const char* scriptPath = "index.js";
    char* scriptContent = readFile("index.js");
    if (scriptContent==NULL) return 1;

    quickjs_initialize();

    s_quick_ctx* pCtx = quick_createContext();
    quickjs_bindFunction(pCtx, "js_print", 1, js_print);
    quickjs_bindFunction(pCtx, "js_stringToArrayBuffer", 1, js_stringToArrayBuffer);
    quickjs_bindFunction(pCtx, "js_arrayBufferToString", 1, js_arrayBufferToString);

    s_quick_execResult res = quickjs_executeScript(pCtx, scriptContent, scriptPath);

    if (res.isException) {
        PROGP_PRINT("ERROR: ");
        PROGP_PRINTLN(res.errorTitle);
        PROGP_PRINTLN(res.errorStackTrace);
    }

    quickjs_decrContext(pCtx);
    quickjs_exit();

    free(scriptContent);
    return 0;
}

#endif // PROGP_STANDALONE
