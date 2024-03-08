#include "quickjs.h"
#include <stdbool.h>

#ifndef QUICKJS_ENGINE_CPP

// -- Doc --
// https://bellard.org/quickjs/quickjs.html
// https://blogs.igalia.com/compilers/2023/06/12/quickjs-an-overview-and-guide-to-adding-a-new-feature/

#define PROGP_PRINT(m) printf("%s", m)
#define PROGP_PRINTLN(m) printf("%s\n", m)
#define PROGP_DEBUG(m) std::cout << "[C-PROGP_DEBUG] - " << m << std::endl
#define PROGP_LOG_ERROR(FROM, WHAT) std::cout << "ERROR - " << FROM << " - " << WHAT << std::endl

typedef struct s_quick_ctx s_quick_ctx;

typedef struct s_quick_execResult {
    bool isSet;
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
} s_quick_ctx;

void quick_initialize();
void quick_exit();

s_quick_ctx* quick_createContext();
void quick_incrContext(s_quick_ctx* pCtx);
void quick_decrContext(s_quick_ctx* pCtx);

s_quick_execResult quick_executeScriptString(s_quick_ctx* pCtx, const char* script, const char* origin);
void quick_bindFunction(s_quick_ctx* pCtx, const char* functionName, int minArgCount, JSCFunction fct);

#endif // QUICKJS_ENGINE_CPP