package main

import (
	"log"

	"goproc/internal/app"

	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "goproc [command]",
		Short: "goproc: small process watcher",
		Long:  `goproc is a small process watcher that can be used to monitor and manage processes.`,
	}
	configPath string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to JSON config file")
}

func controller() *app.App {
	return app.New(app.Options{ConfigPath: configPath})
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
