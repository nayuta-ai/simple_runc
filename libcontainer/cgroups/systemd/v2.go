package systemd

import (
	"github.com/simple_runc/libcontainer/cgroups"
	"github.com/simple_runc/libcontainer/cgroups/fs2"
	"github.com/simple_runc/libcontainer/configs"
)

type UnifiedManager struct {
	// mu      sync.Mutex
	cgroups *configs.Cgroup
	// path is like "/sys/fs/cgroup/user.slice/user-1001.slice/session-1.scope"
	path  string
	dbus  *dbusConnManager
	fsMgr cgroups.Manager
}

func NewUnifiedManager(config *configs.Cgroup, path string) (*UnifiedManager, error) {
	m := &UnifiedManager{
		cgroups: config,
		path:    path,
		dbus:    newDbusConnManager(false),
	}
	if err := m.initPath(); err != nil {
		return nil, err
	}

	fsMgr, err := fs2.NewManager(config, m.path)
	if err != nil {
		return nil, err
	}
	m.fsMgr = fsMgr
	return m, nil
}

// (WIP)
func (m *UnifiedManager) initPath() error {
	return nil
}

func (m *UnifiedManager) Apply(pid int) error {
	// if err := fs2.CreateCgroupPath(m.path, m.cgroups); err != nil {
	// 	return err
	// }
	return nil
}
