package fs

import "github.com/simple_runc/libcontainer/configs"

type NameGroup struct {
	GroupName string
	Join      bool
}

func (s *NameGroup) Name() string {
	return s.GroupName
}

func (s *NameGroup) Apply(path string, _ *configs.Resources, pid int) error {
	if s.Join {
		// Ignore errors if the named cgroup does not exists.
		_ = apply(path, pid)
	}
	return nil
}
