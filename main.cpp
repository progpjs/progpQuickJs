/* File generated automatically by the QuickJS compiler. */

#include <cstring>
#include <cstdio>

#include "quickjs-libc.h"

#define PROGP_PRINT(m) printf("%s\n", m)
#define PROGP_DEBUG(m) std::cout << "[C-PROGP_DEBUG] - " << m << std::endl
#define PROGP_LOG_ERROR(FROM, WHAT) std::cout << "ERROR - " << FROM << " - " << WHAT << std::endl

// https://bellard.org/quickjs/quickjs.html

static JSContext *JS_NewCustomContext(JSRuntime *rt)
{
    JSContext *ctx;
    ctx = JS_NewContext(rt);
    if (!ctx) return nullptr;

    JS_AddIntrinsicBigFloat(ctx);
    JS_AddIntrinsicBigDecimal(ctx);
    JS_AddIntrinsicOperators(ctx);

    JS_EnableBignumExt(ctx, 1);
    JS_AddIntrinsicEval(ctx);
    JS_AddIntrinsicProxy(ctx);

    // JS_AddIntrinsicDate(ctx);
    // JS_AddIntrinsicMapSet(ctx);
    // JS_AddIntrinsicPromise(ctx);
    // JS_AddIntrinsicBigInt(ctx);
    // JS_AddIntrinsicStringNormalize(ctx);
    // JS_AddIntrinsicRegExp(ctx);
    // JS_AddIntrinsicJSON(ctx);
    // JS_AddIntrinsicBaseObjects(ctx);
    // JS_AddIntrinsicTypedArrays(ctx);

    //TODO: comment
    js_init_module_std(ctx, "std");
    js_init_module_os(ctx, "os");

    return ctx;
}

JSValue evalScript(JSContext* ctx, const char* script)  {
    const char* filePath = "<evalScript>";
    return JS_Eval(ctx, script, strlen(script), filePath, JS_EVAL_TYPE_GLOBAL);
}

// To see:
// JS_NewCFunction
// ctx.JS_GetException
// JS_EXCEPTION
// JS_IsException
// https://blogs.igalia.com/compilers/2023/06/12/quickjs-an-overview-and-guide-to-adding-a-new-feature/


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

int main(int argc, char **argv)
{
    //region Create and init the the runtime.

    auto rt = JS_NewRuntime();
    js_std_init_handlers(rt);

    // Allows using Atomics.wait()
    JS_SetCanBlock(rt, 1);

    // For javascript ES6 module loading capacities.
    // Will not be used here since we build all in ore files.
    //
    // JS_SetModuleLoaderFunc(rt, nullptr, js_module_loader, nullptr);

    js_std_set_worker_new_context_func(JS_NewCustomContext);

    //endregion

    //region Create a context

    JSContext* ctx = JS_NewCustomContext(rt);

    // No use of the command line here.
    js_std_add_helpers(ctx, -1, nullptr);

    // Task loop, processing the asynchrones things.
    js_std_loop(ctx);

    //endregion

    //region Declare functions

    auto ctxGlobal = JS_GetGlobalObject(ctx);

    // Indiquer le nombre d'args permet d'optimiser.
    auto minArgCount = 1;
    JS_SetPropertyStr(ctx, ctxGlobal, "js_print", JS_NewCFunction(ctx, js_print, "js_print", minArgCount));

    //endregion

    // ............
    auto res = evalScript(ctx, "console.log(123); js_print(11, 22);'ok'");

    if (JS_IsException(res)) {
        PROGP_PRINT("exception found");
    }
    else if (JS_IsString(res)) {
        auto cString = JS_ToCString(ctx, res);
        PROGP_PRINT(cString);
        JS_FreeCString(ctx, cString);
    }

    JS_FreeValue(ctx, res);

    // ............

    JS_FreeValue(ctx, ctxGlobal);
    JS_FreeContext(ctx);

    // Free the engine.
    js_std_free_handlers(rt);
    JS_FreeRuntime(rt);
    return 0;
}
