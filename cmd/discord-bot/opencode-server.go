package main

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"sync"
)

func RunOpencodeServer(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()

	// run opencode server
	port := strconv.Itoa(AppConfig.OpencodePort)

	cmd := exec.Command("opencode", "serve", "-p", port)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		slog.Error("failed to start opencode server", "error", err)
		os.Exit(1)
	}
	// initialize opencode client
	Opencode()

	slog.Info("opencode server started")

	// wait for cancellation, then kill the process
	<-ctx.Done()
	if err := cmd.Process.Kill(); err != nil {
		slog.Error("failed to kill opencode server", "error", err)
	}
	cmd.Wait() // wait for the process to exit
	slog.Info("opencode server stopped")
}
