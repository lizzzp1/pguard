package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

var colors = []string{"\033[32m", "\033[34m", "\033[35m", "\033[36m", "\033[33m", "\033[31m", "\033[92m", "\033[94m", "\033[95m", "\033[96m"}

func findConfigFile() (string, error) {
	home, _ := os.UserHomeDir()
	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(home, ".config")
	}

	locations := []string{
		"./pguard.yaml",
		"./pguard.yml",
		filepath.Join(xdgConfig, "pguard", "pguard.yaml"),
		filepath.Join(xdgConfig, "pguard", "pguard.yml"),
		filepath.Join(home, ".pguard.yaml"),
		filepath.Join(home, ".pguard.yml"),
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", os.ErrNotExist
}

func main() {
	configPath := flag.String("config", "", "Path to YAML config file (searches: ./pguard.yaml, $XDG_CONFIG_HOME/pguard/pguard.yaml, ~/.pguard.yaml)")
	flag.Parse()

	if *configPath == "" {
		found, err := findConfigFile()
		if err != nil {
			log.Println("Error: No config file found. Searched:")
			log.Println("  - ./pguard.yaml")
			log.Println("  - $XDG_CONFIG_HOME/pguard/pguard.yaml (or ~/.config/pguard/pguard.yaml)")
			log.Println("  - ~/.pguard.yaml")
			log.Println("\nUse --config flag to specify a custom location")
			os.Exit(1)
		}
		configPath = &found
		log.Printf("Using config file: %s", *configPath)
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Printf("Error loading config: %v", err)
		os.Exit(1)
	}

	supervisorCfg := SupervisorConfig{
		MaxRestarts:  cfg.Supervisor.MaxRestarts,
		RestartDelay: cfg.Supervisor.RestartDelay,
		PortTimeout:  cfg.Supervisor.PortTimeout,
	}

	shutdownCh := make(chan struct{})

	services := make([]*Service, len(cfg.Services))
	sup := NewSupervisor(services, supervisorCfg, shutdownCh)

	for i, svcCfg := range cfg.Services {
		svcCfg.Color = colors[i%len(colors)]
		services[i] = NewService(svcCfg, services, shutdownCh, supervisorCfg)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		log.Printf("[%s] Shutdown signal received...", timestamp())
		sup.Shutdown()
	}()

	sup.Run(ctx)

	log.Printf("[%s] All services stopped.", timestamp())
}
