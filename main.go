package main

import (
	"os"
	"svcprobe/pkg/cli"
)

func main() {
	exitCode := cli.Run(os.Args, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}
