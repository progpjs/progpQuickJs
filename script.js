function execute() {
    let a = 5;
    console.log("from my script2");
    console.log("D1")
    let obj = {hello: "world"};
    console.log(obj);
    console.log(JSON.stringify(obj));

    boom()
}

execute()
