package libcontainer

import (
	"fmt"
	"os"
	"os/exec"
)

type filePair struct {
	parent *os.File
	child  *os.File
}

type parentProcess interface {
	start() error
}

type initProcess struct {
	cmd *exec.Cmd
}

func (p *initProcess) start() error {
	err := p.cmd.Start()
	if err != nil {
		return fmt.Errorf("unable to start init: %w", err)
	}
	return nil
}
