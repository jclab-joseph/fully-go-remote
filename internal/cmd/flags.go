package cmd

import (
	"flag"
	"log"
	"os"
	"strings"
)

type AppFlags struct {
	Command string
	Token   *string

	// Server
	ServerListenAddress *string
	DelveListenAddress  *string

	// Client
	Connect *string
	ExeFile string
	RunArgs []string
}

func showUsage() {
	flag.Usage()
	os.Exit(2)
}

func (ctx *AppFlags) ParseFlags() {
	argIndex := 1

	ctx.Token = flag.String("token", "", "authentication token")
	ctx.Connect = flag.String("connect", "", "http address")

	ctx.ServerListenAddress = flag.String("listen", "127.0.0.1:2344", "server listen address")
	ctx.DelveListenAddress = flag.String("delve-listen", "127.0.0.1:2345", "delve listen address")

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(2)
	}

	command := os.Args[argIndex]
	if strings.HasPrefix(command, "-") {
		command = ""
	} else {
		argIndex += 1
	}

	flag.CommandLine.Parse(os.Args[argIndex:])
	argIndex = 0
	if len(command) == 0 {
		if flag.NArg() < 1 {
			flag.Usage()
			os.Exit(2)
		}

		command = flag.Arg(0)
		argIndex = 1
	}

	ctx.Command = command
	if command == "server" {
		// nothing
	} else if command == "exec" {
		if flag.NArg() < (argIndex + 1) {
			log.Println("need executable name")
			showUsage()
		}
		ctx.ExeFile = flag.Arg(argIndex)
		argIndex += 1

		if flag.NArg() >= argIndex {
			ctx.RunArgs = flag.Args()[argIndex:]
		}
	} else {
		log.Println("invalid command:", command)
		showUsage()
	}
}
