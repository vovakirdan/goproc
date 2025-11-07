package main

import (
	"flag"
	"log"

	"goproc/internal/app"
	"goproc/internal/tui"
)

func main() {
	configPath := flag.String("config", "", "Path to JSON config file")
	flag.Parse()

	controller := app.New(app.Options{ConfigPath: *configPath})
	if err := tui.Run(controller); err != nil {
		log.Fatalf("tui exited with error: %v", err)
	}
}
