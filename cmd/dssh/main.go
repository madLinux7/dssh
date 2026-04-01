package main

import "github.com/madLinux7/dssh/internal/cli"

var version = "dev"

func main() {
	cli.Execute(version)
}
