#include <string.h>
#include <stdbool.h>
#include "./quickJsEngine.h"

#include "quickjs-libc.h"

typedef struct s_quickJs_string {
    JSRefCountHeader header; /* must come first, 32-bit */
    uint32_t len : 31;
    uint8_t is_wide_char : 1; /* 0 = 8 bits, 1 = 16 bits characters */
    /* for JS_ATOM_TYPE_SYMBOL: hash = 0, atom_type = 3,
       for JS_ATOM_TYPE_PRIVATE: hash = 1, atom_type = 3
       XXX: could change encoding to have one more bit in hash */
    uint32_t hash : 30;
    uint8_t atom_type : 2; /* != 0 if atom, JS_ATOM_TYPE_x */
    uint32_t hash_next; /* atom_index for JS_ATOM_TYPE_SYMBOL */
#ifdef DUMP_LEAKS
    struct list_head link; /* string list */
#endif
    union {
        uint8_t str8[0]; /* 8 bit strings will get an extra null terminator */
        uint16_t str16[0];
    } u;
} s_quickJs_string;

// Allows to check the ref count of an object when debugging.
typedef struct s_quickJs_withHeader {
    int ref_count;
} s_quickJs_withHeader;

typedef struct s_quickJs_gcItem {
    s_quickJs_withHeader header;
} s_quickJs_gcItem;

static inline void jsStringToCString(JSContext *ctx, const JSValueConst jsValue, q_quick_anyValue* anyValue) {
    s_quickJs_string* jsString = jsValue.u.ptr;

    if (jsString->is_wide_char) {
        const char* cString1 = JS_ToCString(ctx, jsValue);
        const char* cString2 = strdup(cString1);
        JS_FreeCString(ctx, cString1);
        anyValue->voidPtr = (void*)cString2;
        anyValue->mustFree = 1;
    } else {
        anyValue->voidPtr = (void*)jsString->u.str8;
        anyValue->mustFree = 0;
    }
}

//endregion

//region Config

f_quickjs_OnContextDestroyed gEventOnContextReleased;
f_quickjs_OnResourceReleased gEventOnAutoDisposeResourceReleased;

void quickjs_setEventOnAutoDisposeResourceReleased(f_quickjs_OnResourceReleased h) {
    gEventOnAutoDisposeResourceReleased = h;
}

void quickjs_setEventOnContextReleased(f_quickjs_OnContextDestroyed callback) {
    gEventOnContextReleased = callback;
}

//endregion

//region Errors

void freeExecException(s_quick_ctx* pCtx) {
    if (pCtx->hasException) {
        if (pCtx->execException.errorTitle != NULL) {
            JS_FreeCString(pCtx->ctx, pCtx->execException.errorTitle);
            pCtx->execException.errorTitle = NULL;
        }
        if (pCtx->execException.errorStackTrace != NULL) {
            JS_FreeCString(pCtx->ctx, pCtx->execException.errorStackTrace);
            pCtx->execException.errorStackTrace = NULL;
        }
    }
}

s_quick_error* checkExecException(s_quick_ctx* pCtx, JSValue jsRes) {
    if (!JS_IsException(jsRes)) {
        JS_FreeValue(pCtx->ctx, jsRes);
        return NULL;
    }

    quickjs_incrContext(pCtx);
    pCtx->hasException = true;

    JSValue vError = JS_GetException(pCtx->ctx);
    const char* sErrorTitle = JS_ToCString(pCtx->ctx, vError);
    if (sErrorTitle) pCtx->execException.errorTitle = sErrorTitle;

    if (JS_IsError(pCtx->ctx, vError)) {
        JSValue vStack = JS_GetPropertyStr(pCtx->ctx, vError, "stack");
        const char* sErrorStackTrace = JS_ToCString(pCtx->ctx, vStack);
        if (sErrorStackTrace) pCtx->execException.errorStackTrace = sErrorStackTrace;
        JS_FreeValue(pCtx->ctx, vStack);
    }

    JS_FreeValue(pCtx->ctx, vError);
    JS_FreeValue(pCtx->ctx, jsRes);

    return &pCtx->execException;
}

void quickjs_releaseError(s_quick_error* error) {
    if (error!=NULL) {
        quickjs_decrContext(error->pCtx);
    }
}

//endregion

//region Context

void freeInputParams(JSContext *ctx, q_quick_anyValue* values, int maxCount) {
    for (int i=0;i<maxCount;i++) {
        q_quick_anyValue* anyValue = &values[i];

        if (anyValue->mustFree) {
            if (anyValue->valueType == JS_TAG_STRING) {
                JS_FreeCString(ctx, anyValue->voidPtr);
            }
        }
    }
}

void disposeContext(s_quick_ctx* pCtx) {
    if (pCtx->hasException) {
        freeExecException(pCtx);
        pCtx->hasException = false;
    }

    if (pCtx->lastInputParamCount!=0) {
        freeInputParams(pCtx->ctx, pCtx->inputAnyValues, pCtx->lastInputParamCount);
    }
}

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

void quickjs_initialize() {
    gEventOnContextReleased = NULL;
    gEventOnAutoDisposeResourceReleased = NULL;
}

s_quick_ctx* quickjs_createContext(void *userData) {
    JSRuntime* runtime = JS_NewRuntime();

    js_std_set_worker_new_context_func(customQuickJsContext);
    js_std_init_handlers(runtime);
    JS_SetModuleLoaderFunc(runtime, NULL, js_module_loader, NULL);

    JSContext* ctx = customQuickJsContext(runtime);

    // No use of the command line here.
    js_std_add_helpers(ctx, -1, NULL);

    // Task loop, processing the asynchrones things.
    js_std_loop(ctx);

    s_quick_ctx* pCtx = malloc(sizeof(s_quick_ctx));
    pCtx->ctx = ctx;
    pCtx->runtime = runtime;
    pCtx->userData = userData;
    pCtx->hasException = false;
    pCtx->execException.pCtx = pCtx;
    //
    pCtx->inputAnyValues = calloc(15, sizeof(q_quick_anyValue));
    pCtx->lastInputParamCount = 0;

    // Allows to retrieve value.
    JS_SetContextOpaque(ctx, pCtx);

    return pCtx;
}

void quickjs_incrContext(s_quick_ctx* pCtx) {
    pCtx->refCount++;

    if (pCtx->hasException) {
        freeExecException(pCtx);
        pCtx->hasException = false;
    }
}

void quickjs_decrContext(s_quick_ctx* pCtx) {
    pCtx->refCount--;
    if (pCtx->refCount!=0) return;

    if (gEventOnContextReleased!=NULL) {
        gEventOnContextReleased(pCtx);
        if (pCtx->refCount!=0) return;
    }

    if (pCtx->hasException) {
        freeExecException(pCtx);
        pCtx->hasException = false;
    }

    if (pCtx->lastInputParamCount!=0) {
        freeInputParams(pCtx->ctx, pCtx->inputAnyValues, pCtx->lastInputParamCount);
    }

    //s_quickJs_gcItem* gcItem = pCtx->ctx;
    //printf("ref count = %d\n", gcItem->header.ref_count);

    // Releasing a context don't free his memory untile the runtime is destroyed.
    // So we have to release the runtime himsef.
    //
    JS_FreeContext(pCtx->ctx);

    js_std_free_handlers(pCtx->runtime);
    JS_FreeRuntime(pCtx->runtime);

    free(pCtx);
}

s_quick_error* quickjs_executeScript(s_quick_ctx* pCtx, const char* script, const char* origin) {
    quickjs_incrContext(pCtx);

    JSValue jsRes = JS_Eval(pCtx->ctx, script, strlen(script), origin, JS_EVAL_TYPE_GLOBAL);
    s_quick_error* err = checkExecException(pCtx, jsRes);
    quickjs_decrContext(pCtx);
    return err;
}

//endregion

//region Calling functions

s_quick_error* quickjs_callFunctionWithUndefined(s_quick_ctx* pCtx, JSValue* host, int keepAlive) {
    JSValue jsRes = JS_Call(pCtx->ctx, *host, JS_UNDEFINED, 0, NULL);
    if (!keepAlive) quickjs_releaseFunction(pCtx, host);
    return checkExecException(pCtx, jsRes);
}

s_quick_error* quickjs_callFunctionWithAutoReleaseResource2(s_quick_ctx* pCtx, JSValue* host, int keepAlive, uintptr_t res) {
    JSValue args[2];
    args[0] = JS_UNDEFINED;
    args[1] = quickjs_newAutoReleaseResource(pCtx, (void*)res);

    JSValue jsRes = JS_Call(pCtx->ctx, *host, JS_UNDEFINED, 2, args);
    if (!keepAlive) quickjs_releaseFunction(pCtx, host);

    JS_FreeValue(pCtx->ctx, args[1]);
    return checkExecException(pCtx, jsRes);
}

//endregion

void quickjs_bindFunction(s_quick_ctx* pCtx, const char* functionName, int minArgCount, JSCFunction fct) {
    JSValue ctxGlobal = JS_GetGlobalObject(pCtx->ctx);
    JS_SetPropertyStr(pCtx->ctx, ctxGlobal, functionName, JS_NewCFunction(pCtx->ctx, fct, functionName, minArgCount));
    JS_FreeValue(pCtx->ctx, ctxGlobal);
}

s_quick_ctx* quickjs_callParamsToAnyValue(JSContext *ctx, int argc, JSValueConst *argv) {
    s_quick_ctx* pCtx = (s_quick_ctx*)JS_GetContextOpaque(ctx);
    q_quick_anyValue* values = pCtx->inputAnyValues;

    if (pCtx->lastInputParamCount!=0) {
        freeInputParams(ctx, pCtx->inputAnyValues, pCtx->lastInputParamCount);
    }
    //
    pCtx->lastInputParamCount = argc;

    int tag;

    for (int i=0;i<argc;i++) {
        JSValueConst jsValue = argv[i];
        q_quick_anyValue* anyValue = &values[i];

        tag = jsValue.tag;

        if (tag == JS_TAG_UNDEFINED) {
            anyValue->valueType = AnyValueTypeUndefined;
            continue;
        }

        if (tag == JS_TAG_NULL) {
            anyValue->valueType = AnyValueTypeNull;
            continue;
        }

        if (tag == JS_TAG_INT) {
            anyValue->valueType = AnyValueTypeInt32;
            anyValue->size = jsValue.u.int32;
            continue;
        }

        if (tag == JS_TAG_FLOAT64) {
            anyValue->valueType = AnyValueTypeNumber;
            anyValue->number = jsValue.u.float64;
            continue;
        }

        if (tag == JS_TAG_BOOL) {
            anyValue->valueType = AnyValueTypeBoolean;
            anyValue->size = jsValue.u.int32;
            continue;
        }

        if (tag == JS_TAG_STRING) {
            anyValue->valueType = AnyValueTypeString;
            jsStringToCString(ctx, jsValue, anyValue);
            continue;
        }

        if (tag == JS_TAG_OBJECT) {
            if (JS_IsFunction(ctx, jsValue)) {
                anyValue->valueType = AnyValueTypeFunction;
                //
                JSValue *valuePtr = (JSValue *) malloc(sizeof(JSValue));

                // Must duplicate since this value will be automatically released by QuickJS.
                // But most of the time it's used for async, so we must keep the ref ok.
                //
                // WARNING: the ref must be released manually once.
                //          Is automatically done once the function is called.
                //          Otherwise you have to do it manually.
                //
                *valuePtr = JS_DupValue(ctx, jsValue);
                anyValue->voidPtr = valuePtr;
                anyValue->mustFree = 0;
            } else {
                anyValue->valueType = AnyValueTypeJson;
                //
                jsValue = JS_JSONStringify(ctx, jsValue, JS_UNDEFINED, JS_UNDEFINED);
                jsStringToCString(ctx, jsValue, anyValue);
            }
        }

        size_t size;
        uint8_t* buffer = JS_GetArrayBuffer(ctx, &size, jsValue);

        if (buffer!=NULL) {
            anyValue->valueType = AnyValueTypeBuffer;
            anyValue->mustFree = 0;
            anyValue->voidPtr = buffer;
            anyValue->size = size;
            anyValue->mustFree = 0;
        }
    }


    return pCtx;
}

void quickjs_releaseFunction(s_quick_ctx* pCtx, JSValue* host) {
    JS_FreeValue(pCtx->ctx, *host);
    free(host);
}

static void onGcAutoReleaseResource(JSRuntime *rt, void *opaque, void *buffer) {
    if (gEventOnAutoDisposeResourceReleased!=NULL) {
        gEventOnAutoDisposeResourceReleased(opaque);
    }
}

JSValue quickjs_newAutoReleaseResource(s_quick_ctx* pCtx, void* value) {
    return JS_NewArrayBuffer(pCtx->ctx, NULL, 0, onGcAutoReleaseResource, value, false);
}