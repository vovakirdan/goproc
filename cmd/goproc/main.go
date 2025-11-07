package main

import (
	"context"
	"log"
	"time"

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

type controllerAPI interface {
	Ping(ctx context.Context, timeout time.Duration) (string, error)
	Add(ctx context.Context, params app.AddParams) (app.AddResult, error)
	List(ctx context.Context, params app.ListParams) ([]app.Process, error)
	Remove(ctx context.Context, params app.RemoveParams) (app.RemoveResult, error)
	Kill(ctx context.Context, params app.KillParams) (app.KillResult, error)
	Tag(ctx context.Context, params app.TagParams) (app.TagResult, error)
	Group(ctx context.Context, params app.GroupParams) (app.GroupResult, error)
	Reset(ctx context.Context, params app.ResetParams) error
	Status() (app.DaemonStatus, error)
	StopDaemon(force bool) error
	StartDaemon() (*app.DaemonHandle, error)
}

var controllerFactory = func() controllerAPI {
	return app.New(app.Options{ConfigPath: configPath})
}

var _ controllerAPI = (*app.App)(nil)

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to JSON config file")
}

func controller() controllerAPI {
	return controllerFactory()
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
