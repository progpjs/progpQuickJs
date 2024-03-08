#include <string.h>
#include <stdbool.h>
#include "./quickJsEngine.h"

#include "quickjs-libc.h"

static JSContext *customQuickJsContext(JSRuntime *rt)
{
    JSContext *ctx = JS_NewContextRaw(rt);
    if (!ctx) return NULL;

    JS_AddIntrinsicBaseObjects(ctx);
    JS_AddIntrinsicDate(ctx);
    JS_AddIntrinsicEval(ctx);
    JS_AddIntrinsicStringNormalize(ctx);
    JS_AddIntrinsicRegExp(ctx);
    JS_AddIntrinsicJSON(ctx);
    JS_AddIntrinsicProxy(ctx);
    JS_AddIntrinsicMapSet(ctx);
    JS_AddIntrinsicTypedArrays(ctx);
    JS_AddIntrinsicPromise(ctx);
    JS_AddIntrinsicBigInt(ctx);

    return ctx;
}

JSRuntime *gRuntime;

void quick_freeExecResult(s_quick_execResult res);

void quick_exit() {
    // Free the engine.
    js_std_free_handlers(gRuntime);
    JS_FreeRuntime(gRuntime);
}

void quick_initialize() {
    gRuntime = JS_NewRuntime();

    js_std_set_worker_new_context_func(customQuickJsContext);
    js_std_init_handlers(gRuntime);
    JS_SetModuleLoaderFunc(gRuntime, NULL, js_module_loader, NULL);
}

s_quick_ctx* quick_createContext() {
    JSContext* ctx = customQuickJsContext(gRuntime);

    // No use of the command line here.
    js_std_add_helpers(ctx, -1, NULL);

    // Task loop, processing the asynchrones things.
    js_std_loop(ctx);

    s_quick_ctx* pCtx = malloc(sizeof(s_quick_ctx));
    pCtx->ctx = ctx;
    pCtx->hasResult = false;

    // Allows to retrieve value.
    JS_SetContextOpaque(ctx, pCtx);

    return pCtx;
}

void quick_incrContext(s_quick_ctx* pCtx) {
    pCtx->refCount++;

    if (pCtx->hasResult) {
        quick_freeExecResult(pCtx->result);
        pCtx->hasResult = false;
    }
}

void quick_decrContext(s_quick_ctx* pCtx) {
    pCtx->refCount--;

    if (pCtx->refCount==0) {
        if (pCtx->hasResult) {
            quick_freeExecResult(pCtx->result);
        }

        JS_FreeContext(pCtx->ctx);
        free(pCtx);
    }
}

s_quick_execResult quick_executeScriptString(s_quick_ctx* pCtx, const char* script, const char* origin) {
    quick_incrContext(pCtx);

    s_quick_execResult res;
    res.result = JS_Eval(pCtx->ctx, script, strlen(script), origin, JS_EVAL_TYPE_GLOBAL);
    res.pCtx = pCtx;
    res.errorTitle = NULL;

    if (JS_IsException(res.result)) {
        res.isException = true;
        //js_std_dump_error(ctx);

        JSValue exception_val = JS_GetException(pCtx->ctx);
        bool is_error = JS_IsError(pCtx->ctx, exception_val);

        const char* sErrorTitle = JS_ToCString(pCtx->ctx, exception_val);
        if (sErrorTitle) res.errorTitle = sErrorTitle;

        if (is_error) {
            JSValue val = JS_GetPropertyStr(pCtx->ctx, exception_val, "stack");
            const char* sErrorStackTrace = JS_ToCString(pCtx->ctx, val);
            if (sErrorStackTrace) res.errorStackTrace = sErrorStackTrace;
            JS_FreeValue(pCtx->ctx, val);
        }

        JS_FreeValue(pCtx->ctx, exception_val);
    }
    else {
        res.isException = false;
    }

    pCtx->hasResult = true;
    pCtx->result = res;

    return res;
}

void quick_freeExecResult(s_quick_execResult res) {
    if (res.isException) {
        if (res.errorTitle != NULL) JS_FreeCString(res.pCtx->ctx, res.errorTitle);
        if (res.errorStackTrace != NULL) JS_FreeCString(res.pCtx->ctx, res.errorStackTrace);
    }

    JS_FreeValue(res.pCtx->ctx, res.result);
}

void quick_bindFunction(s_quick_ctx* pCtx, const char* functionName, int minArgCount, JSCFunction fct) {
    JSValue ctxGlobal = JS_GetGlobalObject(pCtx->ctx);
    JS_SetPropertyStr(pCtx->ctx, ctxGlobal, functionName, JS_NewCFunction(pCtx->ctx, fct, functionName, minArgCount));
    JS_FreeValue(pCtx->ctx, ctxGlobal);
}

int quick_enqueueJob(s_quick_ctx* pCtx, JSJobFunc *job_func, int argc, JSValueConst *argv) {
    return JS_EnqueueJob(pCtx->ctx, job_func, argc, argv);
}