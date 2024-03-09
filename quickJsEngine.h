#include "quickjs.h"
#include <stdbool.h>

#ifndef QUICKJS_ENGINE_CPP
#define QUICKJS_ENGINE_CPP

// -- Doc --
// https://bellard.org/quickjs/quickjs.html
// https://blogs.igalia.com/compilers/2023/06/12/quickjs-an-overview-and-guide-to-adding-a-new-feature/

#define DEBUG_PRINT(m) printf("C-DEBUG - %s\n", m)
#define DEBUG_PRINT_KeepLine(m) printf("C-DEBUG - %s", m)

#define AnyValueTypeUndefined   0
#define AnyValueTypeNull        1
#define AnyValueTypeInvalid     2
#define AnyValueTypeNumber      3
#define AnyValueTypeString      4
#define AnyValueTypeBoolean     5
#define AnyValueTypeBuffer      6
#define AnyValueTypeFunction    7
#define AnyValueTypeJson        8
#define AnyValueTypeInt32       9

typedef struct q_quick_anyValue {
    int valueType;
    double number;
    void* voidPtr;
    int size;

    // Used when returning a value in order to
    // declare an error. The value is automatically
    // deleted after consumed.
    //
    char* errorMessage;

    // mustFree is used from the Go side.
    // Allows knowing if the value voidPtr must be free.
    //
    int mustFree;
} q_quick_anyValue;

typedef struct s_quick_ctx s_quick_ctx;

typedef struct s_quick_error {
    s_quick_ctx* pCtx;
    const char* errorTitle;
    const char* errorStackTrace;
} s_quick_error;

typedef struct s_quick_ctx {
    int refCount;
    JSContext* ctx;
    void* userData;
    bool hasException;
    s_quick_error execException;
    q_quick_anyValue* inputAnyValues;
    int lastInputParamCount;
} s_quick_ctx;

typedef void (*f_quickjs_OnContextDestroyed)(s_quick_ctx* ctx);
typedef void (*f_quickjs_OnResourceReleased)(void* resource);

//region Config

void quickjs_setEventOnContextReleased(f_quickjs_OnContextDestroyed callback);
void quickjs_setEventOnAutoDisposeResourceReleased(f_quickjs_OnResourceReleased h);

//endregion

//region Context

s_quick_ctx* quick_createContext(void* userData);
void quickjs_incrContext(s_quick_ctx* pCtx);
void quickjs_decrContext(s_quick_ctx* pCtx);

s_quick_error* quickjs_executeScript(s_quick_ctx* pCtx, const char* script, const char* origin);
void quickjs_bindFunction(s_quick_ctx* pCtx, const char* functionName, int minArgCount, JSCFunction fct);

//endregion

//region Engine

void quickjs_initialize();
void quickjs_exit();

//endregion

//region Release ref

void quickjs_releaseError(s_quick_error* error);
void quickjs_releaseFunction(s_quick_ctx* pCtx, JSValue* host);

//endregion

//region Calling functions

s_quick_error* quickjs_callFunctionWithUndefined(s_quick_ctx* pCtx, JSValue* host, int keepAlive);
s_quick_error* quickjs_callFunctionWithAutoReleaseResource2(s_quick_ctx* pCtx, JSValue* host, int keepAlive, uintptr_t res);

//endregion

s_quick_ctx* quickjs_callParamsToAnyValue(JSContext *ctx, int argc, JSValueConst *argv);

JSValue quickjs_newAutoReleaseResource(s_quick_ctx* pCtx, void* value);

#endif // QUICKJS_ENGINE_CPP