package libcontainer

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/simple_runc/libcontainer/cgroups"
)

type filePair struct {
	parent *os.File
	child  *os.File
}

type parentProcess interface {
	start() error
}

type initProcess struct {
	cmd             *exec.Cmd
	messageSockPair filePair
	manager         cgroups.Manager
	bootstrapData   io.Reader
}

func (p *initProcess) pid() int {
	return p.cmd.Process.Pid
}

func (p *initProcess) start() error {
	defer p.messageSockPair.parent.Close()
	err := p.cmd.Start()
	_ = p.messageSockPair.child.Close()
	if err != nil {
		return fmt.Errorf("unable to start init: %w", err)
	}
	waitInit := initWaiter(p.messageSockPair.parent)

	// Apply control group settings to the current process ID.
	// Cgroups, also known as control groups, are a Linux kernel feature that provides a way
	// to limit and allocate system resources such as CPU, memory, and I/O bandwidth among processes or groups of processes.
	// Cgroups are used to manage and isolate system resources for processes,
	// so that a process or group of processes can only use the resources that are allocated to it.
	if err := p.manager.Apply(p.pid()); err != nil {
		return fmt.Errorf("unable to apply cgroup configuration: %w", err)
	}
	if _, err := io.Copy(p.messageSockPair.parent, p.bootstrapData); err != nil {
		return fmt.Errorf("can't copy bootstrap data to pipe: %w", err)
	}

	err = <-waitInit
	if err != nil {
		return err
	}
	return nil
}

func initWaiter(r io.Reader) chan error {
	ch := make(chan error, 1)
	go func() {
		defer close(ch)

		inited := make([]byte, 1)
		_, err := r.Read(inited)
		if err == nil {
			ch <- nil
			return
		}
		ch <- fmt.Errorf("waiting for init preliminary setup: %w", err)
	}()
	return ch
}
