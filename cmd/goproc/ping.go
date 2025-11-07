package main

import (
	"fmt"
	"os"
	"time"

	"goproc/internal/app"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cmdPing)
}

var pingTimeoutSeconds int

func init() {
	cmdPing.Flags().IntVarP(&pingTimeoutSeconds, "timeout", "t", 2, "Timeout in seconds for daemon ping")
}

// `goproc ping` — health-check для демона.
// Поведение:
//  1. Если демон не запущен — возвращаем ошибку.
//  2. Если запущен — делаем gRPC Ping; печатаем "pong" при успехе.
var cmdPing = &cobra.Command{
	Use:   "ping",
	Short: "Check daemon availability (expects 'pong')",
	RunE: func(cmd *cobra.Command, args []string) error {
		controller := app.New(app.Options{ConfigPath: configPath})
		msg, err := controller.Ping(cmd.Context(), time.Duration(pingTimeoutSeconds)*time.Second)
		if err != nil {
			return err
		}

		// Нормальный ответ — "pong"
		fmt.Fprintln(os.Stdout, msg)
		return nil
	},
}
