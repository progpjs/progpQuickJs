#include <string.h>
#include <stdbool.h>
#include "./quickJsEngine.h"

#include "quickjs-libc.h"

JSRuntime *gRuntime;

void freeInputParams(JSContext *ctx, s_progp_anyValue* values, int maxCount);

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

// *********************************************************************************************************************

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

void quickjs_freeExecResult(s_quick_execResult res);

void quickjs_exit() {
    // Free the engine.
    js_std_free_handlers(gRuntime);
    JS_FreeRuntime(gRuntime);
}

void quickjs_initialize() {
    gRuntime = JS_NewRuntime();

    js_std_set_worker_new_context_func(customQuickJsContext);
    js_std_init_handlers(gRuntime);
    JS_SetModuleLoaderFunc(gRuntime, NULL, js_module_loader, NULL);
}

s_quick_ctx* quick_createContext(void *userData) {
    JSContext* ctx = customQuickJsContext(gRuntime);

    // No use of the command line here.
    js_std_add_helpers(ctx, -1, NULL);

    // Task loop, processing the asynchrones things.
    js_std_loop(ctx);

    s_quick_ctx* pCtx = malloc(sizeof(s_quick_ctx));
    pCtx->ctx = ctx;
    pCtx->userData = userData;
    pCtx->hasResult = false;
    pCtx->inputAnyValues = calloc(15, sizeof(s_progp_anyValue));
    pCtx->lastInputParamCount = 0;


    // Allows to retrieve value.
    JS_SetContextOpaque(ctx, pCtx);

    return pCtx;
}

void quickjs_incrContext(s_quick_ctx* pCtx) {
    pCtx->refCount++;

    if (pCtx->hasResult) {
        quickjs_freeExecResult(pCtx->result);
        pCtx->hasResult = false;
    }
}

void quickjs_decrContext(s_quick_ctx* pCtx) {
    pCtx->refCount--;

    if (pCtx->refCount==0) {
        if (pCtx->hasResult) {
            quickjs_freeExecResult(pCtx->result);
        }

        if (pCtx->lastInputParamCount!=0) {
            freeInputParams(pCtx->ctx, pCtx->inputAnyValues, pCtx->lastInputParamCount);
        }

        JS_FreeContext(pCtx->ctx);
        free(pCtx);
    }
}

s_quick_execResult quickjs_executeScript(s_quick_ctx* pCtx, const char* script, const char* origin) {
    quickjs_incrContext(pCtx);

    s_quick_execResult res;
    res.result = JS_Eval(pCtx->ctx, script, strlen(script), origin, JS_EVAL_TYPE_GLOBAL);
    res.pCtx = pCtx;
    res.errorTitle = NULL;

    if (JS_IsException(res.result)) {
        res.isException = true;

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

void quickjs_freeExecResult(s_quick_execResult res) {
    if (res.isException) {
        if (res.errorTitle != NULL) JS_FreeCString(res.pCtx->ctx, res.errorTitle);
        if (res.errorStackTrace != NULL) JS_FreeCString(res.pCtx->ctx, res.errorStackTrace);
    }

    JS_FreeValue(res.pCtx->ctx, res.result);
}

void quickjs_bindFunction(s_quick_ctx* pCtx, const char* functionName, int minArgCount, JSCFunction fct) {
    JSValue ctxGlobal = JS_GetGlobalObject(pCtx->ctx);
    JS_SetPropertyStr(pCtx->ctx, ctxGlobal, functionName, JS_NewCFunction(pCtx->ctx, fct, functionName, minArgCount));
    JS_FreeValue(pCtx->ctx, ctxGlobal);
}

int quickjs_enqueueJob(s_quick_ctx* pCtx, JSJobFunc *job_func, int argc, JSValueConst *argv) {
    return JS_EnqueueJob(pCtx->ctx, job_func, argc, argv);
}

static inline void jsStringToCString(JSContext *ctx, const JSValueConst jsValue, s_progp_anyValue* anyValue) {
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

void freeInputParams(JSContext *ctx, s_progp_anyValue* values, int maxCount) {
    for (int i=0;i<maxCount;i++) {
        s_progp_anyValue* anyValue = &values[i];

        if (anyValue->mustFree) {
            if (anyValue->valueType == JS_TAG_STRING) {
                JS_FreeCString(ctx, anyValue->voidPtr);
            }
        }
    }
}

s_quick_ctx* quickjs_callParamsToAnyValue(JSContext *ctx, int argc, JSValueConst *argv) {
    s_quick_ctx* pCtx = (s_quick_ctx*)JS_GetContextOpaque(ctx);
    s_progp_anyValue* values = pCtx->inputAnyValues;

    if (pCtx->lastInputParamCount!=0) {
        freeInputParams(ctx, pCtx->inputAnyValues, pCtx->lastInputParamCount);
    }
    //
    pCtx->lastInputParamCount = argc;

    int tag;

    for (int i=0;i<argc;i++) {
        JSValueConst jsValue = argv[i];
        s_progp_anyValue* anyValue = &values[i];

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

void quickjs_callFunction(s_quick_ctx* pCtx, JSValue* host) {
    JSValue res = JS_Call(pCtx->ctx, *host, JS_UNDEFINED, 0, NULL);
    JS_FreeValue(pCtx->ctx, res);
}