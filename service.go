package main

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"syscall"
	"time"
)

type ServiceConfig struct {
	Name      string
	Command   string
	Args      []string
	Dir       string
	Port      int
	DependsOn string
}

type Service struct {
	Config        ServiceConfig
	readyChans    map[string]chan struct{}
	cmd           *exec.Cmd
	shutdownCh    chan struct{}
	restartCount  int
	supervisorCfg SupervisorConfig
}

func NewService(cfg ServiceConfig, readyChans map[string]chan struct{}, shutdownCh chan struct{}, supervisorCfg SupervisorConfig) *Service {
	return &Service{
		Config:        cfg,
		readyChans:    readyChans,
		shutdownCh:    shutdownCh,
		supervisorCfg: supervisorCfg,
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.waitForDependencies()

	cmd := exec.CommandContext(ctx, s.Config.Command, s.Config.Args...)

	if s.Config.Dir != "" {
		cmd.Dir = s.Config.Dir
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[%s] Failed to start: %w", s.Config.Name, err)
	}

	s.cmd = cmd
	fmt.Printf("[%s] Started with PID %d\n", s.Config.Name, cmd.Process.Pid)

	if s.Config.Port > 0 {
		s.waitForPort()
	}

	return nil
}

func (s *Service) waitForDependencies() {
	if s.Config.DependsOn == "" {
		return
	}

	fmt.Printf("[%s] Waiting for %s to be ready...\n", s.Config.Name, s.Config.DependsOn)

	if ch, ok := s.readyChans[s.Config.DependsOn]; ok {
		<-ch
	}

	fmt.Printf("[%s] %s is ready, starting...\n", s.Config.Name, s.Config.DependsOn)
}

func (s *Service) waitForPort() {
	go func() {
		defer close(s.readyChans[s.Config.Name])

		if ok := s.waitForPortWithTimeout(); ok {
			fmt.Printf("[%s] Port %d is ready\n", s.Config.Name, s.Config.Port)
			return
		}
		fmt.Printf("[%s] Timeout exceeded waiting for port %d\n", s.Config.Name, s.Config.Port)
	}()
}

func (s *Service) waitForPortWithTimeout() bool {
	timeout := s.supervisorCfg.PortTimeout
	expiry := time.Now().Add(timeout)

	for time.Now().Before(expiry) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", s.Config.Port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}

		select {
		case <-s.shutdownCh:
			return false
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
	return false
}

func (s *Service) Stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		fmt.Printf("[%s] Stopping (PID %d)...\n", s.Config.Name, s.cmd.Process.Pid)
		syscall.Kill(-s.cmd.Process.Pid, syscall.SIGTERM)
	}
}

func (s *Service) PID() int {
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Pid
	}
	return 0
}

func (s *Service) Wait() error {
	if s.cmd != nil {
		return s.cmd.Wait()
	}
	return nil
}

func (s *Service) RestartCount() int {
	return s.restartCount
}

func (s *Service) ShouldRestart(maxRestarts int) bool {
	s.restartCount++
	return s.restartCount <= maxRestarts
}
