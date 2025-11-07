package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"goproc/internal/daemon"
)

func main() {
	configPath := flag.String("config", "", "Path to JSON config file")
	force := flag.Bool("force", false, "Stop an existing daemon before starting")
	flag.Parse()

	if daemon.IsRunning() {
		if !*force {
			pid, err := daemon.RunningPID()
			if err != nil {
				log.Fatalf("daemon appears running but pid check failed: %v", err)
			}
			log.Printf("Daemon is already running (pid %d). Use --force to restart.", pid)
			return
		}
		log.Printf("Stopping existing daemon...")
		if err := daemon.StopRunningDaemon(true); err != nil {
			log.Fatalf("failed to stop running daemon: %v", err)
		}
	}

	srv, err := daemon.StartDaemon(*configPath)
	if err != nil {
		log.Fatalf("failed to start daemon: %v", err)
	}
	log.Printf("Daemon started (pid %d). Press Ctrl+C to stop.", os.Getpid())

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	<-sigc
	log.Printf("Stopping daemon...")
	if err := srv.Close(); err != nil {
		log.Fatalf("error shutting down daemon: %v", err)
	}
	log.Printf("Daemon stopped.")
}
