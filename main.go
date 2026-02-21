package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

var (
	colors   = []string{"\033[32m", "\033[34m", "\033[35m", "\033[36m", "\033[33m", "\033[31m", "\033[92m", "\033[94m", "\033[95m", "\033[96m"}
	colorIdx int
)

func getColor() string {
	color := colors[colorIdx%len(colors)]
	colorIdx++
	return color
}

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
		svcCfg.Color = getColor()
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
