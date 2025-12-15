(() => {
    const {env} = require("process");
    console.println(env.get("PWD"));
})()