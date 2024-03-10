let cb = function() {
    console.log("Inside callback");

    for (let arg of arguments) {
        console.log(arg);
    }
}

js_test(cb, 1234);