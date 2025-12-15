const {sayHello} = require("demo");
const {args} = require("process");

sayHello(args[0]);
