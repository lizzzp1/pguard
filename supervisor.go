package main

import (
	"context"
	"log"
	"os/exec"
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
	log.Printf("[%s] All services stopped.", timestamp())
}

func (s *Supervisor) monitorAndRestart(ctx context.Context, svc *Service) {
	defer s.wg.Done()

	for {
		if s.shouldStop(ctx) {
			svc.Stop()
			return
		}

		if err := svc.Start(ctx); err != nil {
			log.Printf("[%s] %s Failed to start: %v", timestamp(), svc.Config.Name, err)
		}

		err := svc.Wait()
		exitCode := 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				exitCode = ee.ProcessState.ExitCode()
			}
		}
		log.Printf("[%s] %s Process exited with code %d", timestamp(), svc.Config.Name, exitCode)

		if s.shouldStop(ctx) {
			return
		}

		if !svc.ShouldRestart(s.SupervisorConfig.MaxRestarts) {
			log.Printf("[%s] %s Max restarts (%d) exceeded, giving up", timestamp(), svc.Config.Name, s.SupervisorConfig.MaxRestarts)
			return
		}

		log.Printf("[%s] [RESTART] %s Restarting in %v (attempt %d/%d)", timestamp(), svc.Config.Name, s.SupervisorConfig.RestartDelay, svc.RestartCount(), s.SupervisorConfig.MaxRestarts)

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
