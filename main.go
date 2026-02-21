package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	configPath := flag.String("config", "", "Path to YAML config file")
	flag.Parse()

	if *configPath == "" {
		fmt.Println("Error: --config flag is required")
		flag.Usage()
		os.Exit(1)
	}

	cfg, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
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
		services[i] = NewService(svcCfg, shutdownCh, supervisorCfg)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		fmt.Println("\n[pguard] Shutdown signal received...")
		sup.Shutdown()
	}()

	sup.Run(ctx)

	fmt.Println("[pguard] All services stopped.")
}
