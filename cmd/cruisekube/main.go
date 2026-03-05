package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/truefoundry/cruisekube/pkg/logging"

	"github.com/spf13/viper"
	_ "go.uber.org/automaxprocs"
)

var (
	configFilePath string
	v              = viper.New()
)

func main() {
	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	rootCmd := newRootCommand(rootCtx)

	if err := rootCmd.Execute(); err != nil {
		logging.Fatalf(context.Background(), "Cruisekube failed: %v", err)
	}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}
