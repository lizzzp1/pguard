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
	Color     string
}

type Service struct {
	Config        ServiceConfig
	readyChans    map[string]chan struct{}
	cmd           *exec.Cmd
	shutdownCh    chan struct{}
	pty           *PTY
	restartCount  int
	supervisorCfg SupervisorConfig
}

func NewService(cfg ServiceConfig, shutdownCh chan struct{}, supervisorCfg SupervisorConfig) *Service {
	return &Service{
		Config:        cfg,
		shutdownCh:    shutdownCh,
		supervisorCfg: supervisorCfg,
	}
}

func (s *Service) Start(ctx context.Context) error {
	s.waitForDependencies()

	s.readyChans = s.createReadyChan()

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

	fmt.Printf("[%s] Started with PID %d\n", s.Config.Name, cmd.Process.Pid)

	if s.Config.Port > 0 {
		s.waitForPort()
	}

	s.LogOutput(pty)

	return nil
}

func (s *Service) createReadyChan() map[string]chan struct{} {
	readyChans := make(map[string]chan struct{})
	readyChans[s.Config.Name] = make(chan struct{})

	return readyChans
}

func (s *Service) LogOutput(pty *PTY) {
	go func() {
		for {
			line, err := pty.Reader.ReadString('\n')
			if err != nil {
				return
			}
			fmt.Printf("%s[%s]\033[0m %s", s.Config.Color, s.Config.Name, line)
		}
	}()
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
	name := s.Config.Name
	ch := s.readyChans[name]

	go func() {
		defer close(ch)
		if ok := s.waitForPortWithTimeout(); ok {
			fmt.Printf("[%s] Port %d is ready\n", name, s.Config.Port)
			return
		}
		fmt.Printf("[%s] Timeout exceeded waiting for port %d\n", name, s.Config.Port)
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
	if s.pty != nil {
		s.pty.Close()
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
