package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"syscall"
	"time"
)

func timestamp() string {
	return time.Now().Format("15:04:05")
}

type ServiceConfig struct {
	Name      string
	Command   string
	Args      []string
	Dir       string
	Port      int
	Host      string
	DependsOn string
	Color     string
}

type Service struct {
	Config        ServiceConfig
	services      []*Service
	readyChan     chan struct{}
	cmd           *exec.Cmd
	shutdownCh    chan struct{}
	pty           *PTY
	restartCount  int
	supervisorCfg SupervisorConfig
}

func NewService(cfg ServiceConfig, services []*Service, shutdownCh chan struct{}, supervisorCfg SupervisorConfig) *Service {
	return &Service{
		Config:        cfg,
		services:      services,
		shutdownCh:    shutdownCh,
		supervisorCfg: supervisorCfg,
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.waitForDependencies()

	s.readyChan = s.createReadyChan()

	cmd := exec.CommandContext(ctx, s.Config.Command, s.Config.Args...)

	if s.Config.Dir != "" {
		cmd.Dir = s.Config.Dir
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	pty, err := SetupPTY(cmd)

	if err != nil {
		return fmt.Errorf("[%s] Failed to start: %w", s.Config.Name, err)
	}

	s.cmd = cmd
	s.pty = pty

	dir := s.Config.Dir
	if dir == "" {
		dir = "."
	}
	log.Printf("[%s] %s Starting: %s %v (dir: %s)", timestamp(), s.Config.Name, s.Config.Command, s.Config.Args, dir)
	log.Printf("[%s] %s Started with PID %d", timestamp(), s.Config.Name, cmd.Process.Pid)

	if s.Config.Port > 0 {
		s.waitForPort()
	}

	s.LogOutput(pty)

	return nil
}

func (s *Service) createReadyChan() chan struct{} {
	return make(chan struct{})
}

func (s *Service) LogOutput(pty *PTY) {
	go func() {
		for {
			line, err := pty.Reader.ReadString('\n')
			if err != nil {
				return
			}
			log.Printf("%s %s[%s]\033[0m %s", timestamp(), s.Config.Color, s.Config.Name, line)
		}
	}()
}

func (s *Service) waitForDependencies() {
	if s.Config.DependsOn == "" {
		return
	}

	log.Printf("[%s] %s Waiting for %s to be ready...", timestamp(), s.Config.Name, s.Config.DependsOn)

	for _, svc := range s.services {
		if svc.Config.Name == s.Config.DependsOn {
			<-svc.readyChan
			break
		}
	}

	log.Printf("[%s] %s %s is ready, starting...", timestamp(), s.Config.Name, s.Config.DependsOn)
}

func (s *Service) waitForPort() {
	go func() {
		defer close(s.readyChan)
		if ok := s.waitForPortWithTimeout(); ok {
			log.Printf("[%s] %s Port %d is ready", timestamp(), s.Config.Name, s.Config.Port)
			return
		}
		log.Printf("[%s] %s Timeout exceeded waiting for port %d", timestamp(), s.Config.Name, s.Config.Port)
	}()
}

func (s *Service) waitForPortWithTimeout() bool {
	timeout := s.supervisorCfg.PortTimeout
	expiry := time.Now().Add(timeout)

	for time.Now().Before(expiry) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port), 500*time.Millisecond)
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
		log.Printf("[%s] %s Stopping (PID %d)...", timestamp(), s.Config.Name, s.cmd.Process.Pid)
		syscall.Kill(-s.cmd.Process.Pid, syscall.SIGTERM)
		s.cmd.Wait()
	}
	s.pty.Close()
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
