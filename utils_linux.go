package main

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/simple_runc/libcontainer"
	"github.com/simple_runc/libcontainer/specconv"
	"github.com/urfave/cli"
)

func newProcess() (*libcontainer.Process, error) {
	return &libcontainer.Process{}, nil
}

func createContainer(context *cli.Context, id string, spec *specs.Spec) (*libcontainer.Container, error) {
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName: id,
		Spec:       spec,
	})
	if err != nil {
		return nil, err
	}
	// root := context.GlobalString("root")
	return libcontainer.Create(id, config)
}

type runner struct {
	init      bool
	container *libcontainer.Container
}

func (r *runner) run() (int, error) {
	process, err := newProcess()
	if err != nil {
		return -1, err
	}
	if err := r.container.Start(process); err != nil {
		return -1, err
	}
	return 0, nil
}

func startContainer(context *cli.Context) (int, error) {
	id := ""
	container, err := createContainer(context, id, &specs.Spec{})
	if err != nil {
		return -1, err
	}
	r := &runner{
		init:      true,
		container: container,
	}
	return r.run()
}
