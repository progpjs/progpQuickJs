#include <stdio.h>
#include <stdlib.h>
#include <stdbool.h>
#include <string.h>
#include "quickJsEngine.h"

// To see:
// ctx.JS_GetException
// JS_EXCEPTION
// JS_IsException
// Async: https://www.freelists.org/post/quickjs-devel/Unable-to-await-promise,1

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
    const char* scriptPath = "script.js";
    char* scriptContent = readFile("script.js");
    if (scriptContent==NULL) return 1;

    quick_initialize();

    s_quick_ctx* pCtx = quick_createContext();
    quick_bindFunction(pCtx, "js_print", 1, js_print);

    s_quick_execResult res = quick_executeScriptString(pCtx, scriptContent, scriptPath);

    if (res.isException) {
        PROGP_PRINT("ERROR: ");
        PROGP_PRINTLN(res.errorTitle);
        PROGP_PRINTLN(res.errorStackTrace);
    }

    quick_decrContext(pCtx);
    quick_exit();

    free(scriptContent);
    return 0;
}
