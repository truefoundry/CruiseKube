package main

import (
	"context"
	"github.com/spf13/viper"
	_ "go.uber.org/automaxprocs"
	"os"
)

var (
	configFilePath string
	v              = viper.New()
)

func main() {
	ctx := context.Background()
	rootCmd := newRootCommand(ctx)
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
