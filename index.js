function test(title, v) {
    console.log(title, "- Send:", v, "- Received:", js_decodeParams(v));
}

test("string", "my string");
test("int", 123);
test("float", 1.23);
test("bool(true)", true);
test("bool(false)", false);
