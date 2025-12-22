package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/OutOfBedlam/jsh/engine"
	"github.com/OutOfBedlam/jsh/native/http"
	"github.com/OutOfBedlam/jsh/native/mqtt"
	"github.com/OutOfBedlam/jsh/native/readline"
	"github.com/OutOfBedlam/jsh/native/shell"
	"github.com/OutOfBedlam/jsh/native/ws"
)

// JSH options:
//  1. -c "script" : command to execute
//     ex: jsh -c "console.println(require('/lib/process').argv[2])" helloworld
//  2. script file : execute script file
//     ex: jsh script.js arg1 arg2
//  3. no args : start interactive shell
//     ex: jsh
func main() {
	var fstabs engine.FSTabs
	src := flag.String("c", "", "command to execute")
	scf := flag.String("s", "", "configured file to start from")
	flag.Var(&fstabs, "v", "volume to mount (format: /mountpoint=source)")
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
		conf.FSTabs = fstabs
		conf.Args = flag.Args()
		conf.Default = "/sbin/shell.js" // default script to run if no args
		conf.Env = map[string]any{
			"PATH": "/sbin:/lib:/work",
			"HOME": "/work",
			"PWD":  "/work",
		}
	}
	engine, err := engine.New(conf)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	engine.RegisterNativeModule("@jsh/process", engine.Process)
	engine.RegisterNativeModule("@jsh/shell", shell.Module)
	engine.RegisterNativeModule("@jsh/readline", readline.Module)
	engine.RegisterNativeModule("@jsh/http", http.Module)
	engine.RegisterNativeModule("@jsh/ws", ws.Module)
	engine.RegisterNativeModule("@jsh/mqtt", mqtt.Module)

	os.Exit(engine.Main())
}
