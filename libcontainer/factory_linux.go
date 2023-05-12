package libcontainer

import (
	"github.com/simple_runc/libcontainer/cgroups/manager"
	"github.com/simple_runc/libcontainer/configs"
)

const execFifoFilename = "exec.fifo"

func Create(id string, config *configs.Config) (*Container, error) {
	cm, err := manager.New(config.Cgroups)
	if err != nil {
		return nil, err
	}
	c := &Container{
		id:            id,
		config:        config,
		cgroupManager: cm,
	}
	return c, nil
}
