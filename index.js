let cb = function() {
    console.log("Inside callback");

    for (let arg of arguments) {
        console.log(arg);
    }

    return 111;
}

js_test(cb, 1234);