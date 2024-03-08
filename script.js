function test1() {
    let a = 5;
    console.log("from my script2");
    console.log("D1")
    let obj = {hello: "world"};
    console.log(obj);
    console.log(JSON.stringify(obj));

    boom()
}

function test2() {
    let buffer = js_stringToArrayBuffer("hello world");
    console.log(js_arrayBufferToString(buffer));
}

test2();
