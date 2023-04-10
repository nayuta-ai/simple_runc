package fs2

import (
	"fmt"
	"strings"

	"github.com/simple_runc/libcontainer/cgroups"
	"github.com/simple_runc/libcontainer/configs"
)

func supportControllers() (string, error) {
	return cgroups.ReadFile(UnifiedMountpoint, "/cgroup.controllers")
}

func CreateCgroupPath(path string, c *configs.Cgroup) (Err error) {
	if !strings.HasPrefix(path, UnifiedMountpoint) {
		return fmt.Errorf("invalid cgroup path %s", path)
	}

	content, err := supportControllers()
	if err != nil {
		return err
	}

	const (
		cgTypeFile  = "cgroup.type"
		cgStCtlFile = "cgroup.subtree_control"
	)
	ctrs := strings.Fields(content)
	_ = "+" + strings.Join(ctrs, " +")

	return nil
}
