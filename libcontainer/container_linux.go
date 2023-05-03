package libcontainer

import "github.com/simple_runc/libcontainer/utils"

type Container struct{}

// Create new init process.
func (c *Container) newInitProcess(messageSockPair filePair) (*initProcess, error) {
	return &initProcess{}, nil
}

func (c *Container) newParentProcess(p *Process) (parentProcess, error) {
	parentInitPipe, childInitPipe, err := utils.NewSockPair("init")
	if err != nil {
		return nil, err
	}
	messageSockPair := filePair{parentInitPipe, childInitPipe}
	return c.newInitProcess(messageSockPair)
}

func (c *Container) Start(process *Process) error {
	if err := c.start(process); err != nil {
		return err
	}
	return nil
}

func (c *Container) start(process *Process) error {
	parent, err := c.newParentProcess(process)
	if err != nil {
		return err
	}
	if err := parent.start(); err != nil {
		return err
	}
	return nil
}
