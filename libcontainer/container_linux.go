package libcontainer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/simple_runc/libcontainer/cgroups"
	"github.com/simple_runc/libcontainer/configs"
	"github.com/simple_runc/libcontainer/utils"
	"golang.org/x/sys/unix"
)

type netlinkError struct{ error }

const stdioFdCount = 3

type Container struct {
	id            string
	root          string
	fifo          *os.File
	config        *configs.Config
	cgroupManager cgroups.Manager
}

// Create new init process.
func (c *Container) newInitProcess(messageSockPair filePair) (*initProcess, error) {
	c.bootstrapData()
	return &initProcess{}, nil
}

func (c *Container) newParentProcess(p *Process) (parentProcess, error) {
	parentInitPipe, childInitPipe, err := utils.NewSockPair("init")
	if err != nil {
		return nil, err
	}
	messageSockPair := filePair{parentInitPipe, childInitPipe}
	cmd := c.commandTemplate(p, childInitPipe)
	// Call "runc exe"
	if !p.Init {
		return nil, nil
	}
	// Open runc exe fd
	if err := c.includeExecFifo(cmd); err != nil {
		return nil, err
	}
	// Set up init process structure
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

func (c *Container) includeExecFifo(cmd *exec.Cmd) error {
	fifoName := filepath.Join(c.root, execFifoFilename)
	fifo, err := os.OpenFile(fifoName, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	c.fifo = fifo

	cmd.ExtraFiles = append(cmd.ExtraFiles, fifo)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_FIFOFD="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1))
	return nil
}

func (c *Container) commandTemplate(p *Process, childInitPipe *os.File) *exec.Cmd {
	cmd := exec.Command("/proc/self/exe", "init")
	// cmd.Args[0] = os.Args[0]
	// cmd.Stdin = p.Stdin
	cmd.Stdout = p.Stdout
	// cmd.Stderr = p.Stderr
	cmd.Dir = c.root
	cmd.ExtraFiles = append(cmd.ExtraFiles, p.ExtraFiles...)

	cmd.ExtraFiles = append(cmd.ExtraFiles, childInitPipe)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_INITPIPE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1),
		"_LIBCONTAINER_STATEDIR="+c.root,
	)
	return cmd
}

func (c *Container) bootstrapData() {}
