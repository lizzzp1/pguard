package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

var colors = []string{"\033[32m", "\033[34m", "\033[35m", "\033[36m", "\033[33m", "\033[31m", "\033[92m", "\033[94m", "\033[95m", "\033[96m"}

func main() {
	configPath := flag.String("config", "", "Path to YAML config file")
	flag.Parse()

	if *configPath == "" {
		log.Println("Error: --config flag is required")
		flag.Usage()
		os.Exit(1)
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
