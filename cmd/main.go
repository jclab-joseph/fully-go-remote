package main

import (
	"github.com/jc-lab/fully-go-remote/cmd/client"
	"github.com/jc-lab/fully-go-remote/cmd/server"
	"github.com/jc-lab/fully-go-remote/internal/cmd"
)

func main() {
	appFlags := &cmd.AppFlags{}
	appFlags.ParseFlags()

	if appFlags.Command == "server" {
		server.DoServer(appFlags)
	} else if appFlags.Command == "exec" {
		client.DoExec(appFlags)
	}
}
