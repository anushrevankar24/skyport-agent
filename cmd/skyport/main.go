package main

import (
	"log"
	"skyport-agent/internal/cli"
)

func main() {
	// Configuration is baked into the binary at build time via ldflags
	// Environment variables can still override if needed
	if err := cli.Execute(); err != nil {
		log.Fatal(err)
	}
}
