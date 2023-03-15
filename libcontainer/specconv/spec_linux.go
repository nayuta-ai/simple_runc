package specconv

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/simple_runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

type CreateOpts struct {
	CgroupName       string
	NoNewKeyring     bool
	NoPivotRoot      bool
	Spec             *specs.Spec
	UseSystemdCgroup bool
}

// getwd is a wrapper similar to os.Getwd, except it always gets
// the value from the kernel, which guarantees the returned value
// to be absolute and clean.
func getwd() (wd string, err error) {
	for {
		// This function is a wrapper around the 'getcwd' system call,
		// which is used to retrieve the pathname of the current working directory.
		wd, err = unix.Getwd()
		//nolint:errorlint // unix errors are bare
		if err != unix.EINTR {
			break
		}
	}
	return wd, os.NewSyscallError("getwd", err)
}

// CreateLibcontainerConfig creates a new libcontainer configuration from a
// given specification and a cgroup name
func CreateLibcontainerConfig(opts *CreateOpts) (*configs.Config, error) {
	// runc's cwd will always be the bundle path
	// A bundle path is a directory that contains all the files and configuration required
	// to run a container using the Open Container Initiative (OCI) runtime specification.
	cwd, err := getwd()
	if err != nil {
		return nil, err
	}
	spec := opts.Spec
	if spec.Root == nil {
		return nil, errors.New("root must be specified")
	}
	rootfsPath := spec.Root.Path
	if !filepath.IsAbs(rootfsPath) {
		rootfsPath = filepath.Join(cwd, rootfsPath)
	}
	labels := []string{}
	for k, v := range spec.Annotations {
		labels = append(labels, k+"="+v)
	}
	config := &configs.Config{
		Rootfs:      rootfsPath,
		NoPivotRoot: opts.NoPivotRoot,
		Readonlyfs:  spec.Root.Readonly,
		Hostname:    spec.Hostname,
		Labels:      append(labels, "bundle="+cwd),
	}
	return config, nil
}
