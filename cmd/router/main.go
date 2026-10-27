package main

import (
	"flag"
	"fmt"
	"os"
)

var version = "0.1.0-dev"

func main() {
	showVersion := flag.Bool("version", false, "show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("llm-router version", version)
		os.Exit(0)
	}

	fmt.Println("llm-router v" + version)
	fmt.Println("Usage: llm-router [--version]")
}