package main

import (
	"bufio"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

type PTY struct {
	File   *os.File
	Reader *bufio.Reader
}

func SetupPTY(cmd *exec.Cmd) (*PTY, error) {
	file, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	return &PTY{
		File:   file,
		Reader: bufio.NewReader(file),
	}, nil
}

func (p *PTY) Close() error {
	if p.File != nil {
		return p.File.Close()
	}
	return nil
}
