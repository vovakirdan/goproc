package main

import (
	"log"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "goproc [command]",
	Short: "goproc: small process watcher",
	Long:  `goproc is a small process watcher that can be used to monitor and manage processes.`,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
