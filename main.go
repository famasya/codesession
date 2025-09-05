package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func setLogLevel(levelStr string) {
	var level slog.Level
	switch levelStr {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo // default to info
	}

	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))
}

func main() {
	err := LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return
	}

	setLogLevel(AppConfig.LogLevel)
	slog.Info("log level", "level", AppConfig.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	wg.Add(2)
	go RunOpencodeServer(ctx, &wg)
	go RunDiscordBot(ctx, &wg)

	// receive signal
	sig := <-sigs
	slog.Info("received signal", "signal", sig)
	cancel()

	wg.Wait()
	slog.Info("exited")
}
