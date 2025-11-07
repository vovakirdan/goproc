package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	goprocv1 "goproc/api/proto/goproc/v1"
	"goproc/internal/daemon"

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
		// Fast path: короткая проверка наличия сокета и здоровья
		if !daemon.IsRunning() {
			return errors.New("daemon is not running")
		}

		if pingTimeoutSeconds <= 0 {
			return errors.New("timeout must be greater than 0 seconds")
		}

		// gRPC Ping с таймаутом
		ctx, cancel := context.WithTimeout(cmd.Context(), time.Duration(pingTimeoutSeconds)*time.Second)
		defer cancel()

		client, conn, err := daemon.Dial(ctx)
		if err != nil {
			return fmt.Errorf("connect to daemon: %w", err)
		}
		defer conn.Close()

		resp, err := client.Ping(ctx, &goprocv1.PingRequest{})
		if err != nil {
			return fmt.Errorf("daemon ping RPC failed: %w", err)
		}

		// Нормальный ответ — "pong"
		fmt.Fprintln(os.Stdout, resp.GetOk())
		return nil
	},
}
