package main

import (
	"context"
	"os"

	"github.com/spf13/viper"
	_ "go.uber.org/automaxprocs"
)

var (
	configFilePath string
	v              = viper.New()
)

func main() {
	ctx := context.Background()
	rootCmd := newRootCommand(ctx)
	rootCmd.SetContext(ctx)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func blockForever() {
	select {}
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE")
}
