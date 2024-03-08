#include "quickjs.h"
#include <stdbool.h>

#ifndef QUICKJS_ENGINE_CPP
#define QUICKJS_ENGINE_CPP

// -- Doc --
// https://bellard.org/quickjs/quickjs.html
// https://blogs.igalia.com/compilers/2023/06/12/quickjs-an-overview-and-guide-to-adding-a-new-feature/

#define PROGP_PRINT(m) printf("%s", m)
#define PROGP_PRINTLN(m) printf("%s\n", m)
#define PROGP_DEBUG(m) std::cout << "[C-PROGP_DEBUG] - " << m << std::endl
#define PROGP_LOG_ERROR(FROM, WHAT) std::cout << "ERROR - " << FROM << " - " << WHAT << std::endl

#define AnyValueTypeUndefined   0;
#define AnyValueTypeNull        1;
#define AnyValueTypeInvalid     2;
#define AnyValueTypeNumber      3;
#define AnyValueTypeString      4;
#define AnyValueTypeBoolean     5;
#define AnyValueTypeBuffer      6;
#define AnyValueTypeFunction    7;
#define AnyValueTypeJson        8;
#define AnyValueTypeInt32       9;

typedef struct s_progp_anyValue {
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
} s_progp_anyValue;

typedef struct s_quick_ctx s_quick_ctx;

typedef struct s_quick_execResult {
    s_quick_ctx* pCtx;
    uint8_t isException;
    JSValue result;
    const char* errorTitle;
    const char* errorStackTrace;
} s_quick_execResult;

typedef struct s_quick_ctx {
    JSContext* ctx;
    int refCount;
    bool hasResult;
    s_quick_execResult result;
    s_progp_anyValue* inputAnyValues;
} s_quick_ctx;

void quickjs_initialize();
void quickjs_exit();

s_quick_ctx* quick_createContext();
void quickjs_incrContext(s_quick_ctx* pCtx);
void quickjs_decrContext(s_quick_ctx* pCtx);

JSValue quick_createExternalPointer(s_quick_ctx* pCtx, void* ptr);

s_quick_execResult quickjs_executeScript(s_quick_ctx* pCtx, const char* script, const char* origin);
void quickjs_bindFunction(s_quick_ctx* pCtx, const char* functionName, int minArgCount, JSCFunction fct);

s_quick_ctx* quickjs_callParamsToAnyValue(JSContext *ctx, int argc, JSValueConst *argv);

#endif // QUICKJS_ENGINE_CPP