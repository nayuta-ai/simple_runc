package fs2

import "github.com/simple_runc/libcontainer/configs"

type Manager struct {
	config *configs.Cgroup
	// dirPath is like "/sys/fs/cgroup/user.slice/user-1001.slice/session-1.scope"
	dirPath string
	// controllers is content of "cgroup.controllers" file.
	// excludes pseudo-controllers ("devices" and "freezer").
	controllers map[string]struct{}
}

// NewManager creates a manager for cgroup v2 unified hierarchy.
// dirPath is like "/sys/fs/cgroup/user.slice/user-1001.slice/session-1.scope".
// If dirPath is empty, it is automatically set using config.
func NewManager(config *configs.Cgroup, dirPath string) (*Manager, error) {
	m := &Manager{
		config:  config,
		dirPath: dirPath,
	}
	return m, nil
}

func (m *Manager) Apply(pid int) error {
	return nil
}
