/* File generated automatically by the QuickJS compiler. */

#include "quickjs-libc.h"

const uint32_t qjsc_hello_size = 72;

const uint8_t qjsc_hello[72] = {
        0x02, 0x04, 0x0e, 0x63, 0x6f, 0x6e, 0x73, 0x6f,
        0x6c, 0x65, 0x06, 0x6c, 0x6f, 0x67, 0x0a, 0x68,
        0x65, 0x6c, 0x6c, 0x6f, 0x10, 0x68, 0x65, 0x6c,
        0x6c, 0x6f, 0x2e, 0x6a, 0x73, 0x0e, 0x00, 0x06,
        0x00, 0xa2, 0x01, 0x00, 0x01, 0x00, 0x03, 0x00,
        0x00, 0x14, 0x01, 0xa4, 0x01, 0x00, 0x00, 0x00,
        0x38, 0xe3, 0x00, 0x00, 0x00, 0x42, 0xe4, 0x00,
        0x00, 0x00, 0x04, 0xe5, 0x00, 0x00, 0x00, 0x24,
        0x01, 0x00, 0xcf, 0x28, 0xcc, 0x03, 0x01, 0x00,
};

static JSContext *JS_NewCustomContext(JSRuntime *rt)
{
    JSContext *ctx = JS_NewContextRaw(rt);
    if (!ctx)
        return NULL;
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

int main(int argc, char **argv)
{
    JSRuntime *rt;
    JSContext *ctx;
    rt = JS_NewRuntime();
    js_std_set_worker_new_context_func(JS_NewCustomContext);
    js_std_init_handlers(rt);
    JS_SetModuleLoaderFunc(rt, NULL, js_module_loader, NULL);
    ctx = JS_NewCustomContext(rt);
    js_std_add_helpers(ctx, argc, argv);
    js_std_eval_binary(ctx, qjsc_hello, qjsc_hello_size, 0);
    js_std_loop(ctx);
    js_std_free_handlers(rt);
    JS_FreeContext(ctx);
    JS_FreeRuntime(rt);
    return 0;
}
