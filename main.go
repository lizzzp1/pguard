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

	readyChans := make(map[string]chan struct{})
	for _, svcCfg := range cfg.Services {
		readyChans[svcCfg.Name] = make(chan struct{})
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	shutdownCh := make(chan struct{})

	services := make([]*Service, len(cfg.Services))
	for i, svcCfg := range cfg.Services {
		services[i] = NewService(svcCfg, readyChans, shutdownCh, supervisorCfg)
	}

	sup := NewSupervisor(services, readyChans, supervisorCfg)

	go func() {
		<-ctx.Done()
		fmt.Println("\n[pguard] Shutdown signal received...")
		sup.Shutdown()
	}()

	sup.Run(ctx)

	fmt.Println("[pguard] All services stopped.")
}
