package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Supervisor struct {
	services []*Service
	shutdown chan struct{}
	wg       sync.WaitGroup
	SupervisorConfig
}

func NewSupervisor(services []*Service, cfg SupervisorConfig, shutdown chan struct{}) *Supervisor {
	return &Supervisor{
		services:         services,
		SupervisorConfig: cfg,
		shutdown:         shutdown,
	}
}

func (s *Supervisor) Run(ctx context.Context) {
	for _, svc := range s.services {
		s.wg.Add(1)
		go s.monitorAndRestart(ctx, svc)
	}

	s.wg.Wait()
	fmt.Println("[pguard] All services stopped.")
}

func (s *Supervisor) monitorAndRestart(ctx context.Context, svc *Service) {
	defer s.wg.Done()

	for {
		if s.shouldStop(ctx) {
			svc.Stop()
			return
		}

		if err := svc.Start(ctx); err != nil {
			fmt.Printf("[%s] Failed to start: %v\n", svc.Config.Name, err)
		}

		err := svc.Wait()
		if err != nil {
			fmt.Printf("[%s] Process exited: %v\n", svc.Config.Name, err)
		}

		if s.shouldStop(ctx) {
			return
		}

		if !svc.ShouldRestart(s.SupervisorConfig.MaxRestarts) {
			fmt.Printf("[%s] Max restarts (%d) exceeded, giving up\n", svc.Config.Name, s.SupervisorConfig.MaxRestarts)
			s.Shutdown()
			return
		}

		fmt.Printf("[%s] Restarting in %v (attempt %d/%d)\n", svc.Config.Name, s.SupervisorConfig.RestartDelay, svc.RestartCount(), s.SupervisorConfig.MaxRestarts)

		select {
		case <-s.shutdown:
			return
		case <-ctx.Done():
			return
		case <-time.After(s.SupervisorConfig.RestartDelay):
		}
	}
}

func (s *Supervisor) shouldStop(ctx context.Context) bool {
	select {
	case <-s.shutdown:
		return true
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func (s *Supervisor) Shutdown() {
	close(s.shutdown)
}
