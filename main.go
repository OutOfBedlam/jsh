package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OutOfBedlam/jsh/engine"
	"github.com/OutOfBedlam/jsh/native/http"
	"github.com/OutOfBedlam/jsh/native/mach"
	"github.com/OutOfBedlam/jsh/native/mqtt"
	"github.com/OutOfBedlam/jsh/native/readline"
	"github.com/OutOfBedlam/jsh/native/shell"
	"github.com/OutOfBedlam/jsh/native/ws"
)

// JSH options:
//  1. -c "script" : command to execute
//     ex: jsh -c "console.println(require('process').argv[2])" helloworld
//  2. script file : execute script file
//     ex: jsh script.js arg1 arg2
//  3. no args : start interactive shell
//     ex: jsh
func main() {
	src := flag.String("c", "", "command to execute")
	dir := flag.String("d", ".", "working directory")
	scf := flag.String("s", "", "configured file to start from")
	dev := flag.String("dev", "", "use development filesystem")
	flag.Parse()

	conf := engine.Config{}
	if *scf != "" {
		// when it starts with "-s", read secret box
		if err := engine.ReadSecretBox(*scf, &conf); err != nil {
			fmt.Println("Error reading secret file:", err.Error())
			os.Exit(1)
		}
	} else {
		// otherwise, use command args to build ExecPass
		conf.Code = *src
		conf.Dir = *dir
		conf.Dev = *dev
		conf.Args = flag.Args()
		conf.Default = "/sbin/shell.js" // default script to run if no args
	}
	engine, err := engine.New(conf)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	engine.RegisterNativeModule("process", engine.Process)
	engine.RegisterNativeModule("shell", shell.Module)
	engine.RegisterNativeModule("machcli", mach.Module)
	engine.RegisterNativeModule("readline", readline.Module)
	engine.RegisterNativeModule("http", http.Module)
	engine.RegisterNativeModule("ws", ws.Module)
	engine.RegisterNativeModule("mqtt", mqtt.Module)

	os.Exit(engine.Main())
}
