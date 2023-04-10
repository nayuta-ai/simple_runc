package manager

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/simple_runc/libcontainer/cgroups"
	"github.com/simple_runc/libcontainer/cgroups/fs"
	"github.com/simple_runc/libcontainer/cgroups/fs2"
	"github.com/simple_runc/libcontainer/cgroups/systemd"
	"github.com/simple_runc/libcontainer/configs"
)

func New(config *configs.Cgroup) (cgroups.Manager, error) {
	return NewWithPaths(config, nil)
}

// There is four kind of a manager such as fs.NewManager, systemd.NewLegacyManager, fs2.NewManager, and systemd.NewUnifiedManager.
// If your system is any linux distribution such as centos, ubuntu, etc., it uses systemd.
// fs.NewManager creates an instance of a control group manager that uses the original control group filesystem (cgroup v1) to manage control groups.
// It provides basic functionality for managing control groups, such as creating and removing control groups, updating resource limits, and monitoring resource usage.
// fs2.NewManager creates an instance of a control group manager that uses the unified control group filesystem (cgroup v2) to manage control groups.
// It provides a more modern and feature-rich implementation of control group management, with support for unified control over all system resources, better performance, and more flexible configuration.
// This function uses systemd.NewUnifiedManager because it supports cgourp2 which is efficient and suppose that it runs on Ubuntu.
func NewWithPaths(config *configs.Cgroup, paths map[string]string) (cgroups.Manager, error) {
	if config == nil {
		return nil, errors.New("cgroups/manager.New: config must not be nil")
	}
	if config.Systemd && !systemd.IsRunningSystemd() {
		return nil, errors.New("systemd not running on this host, cannot use systemd cgroups manager")
	}
	if cgroups.IsCgroup2UnifiedMode() {
		path, err := getunifiedPath(paths)
		if err != nil {
			return nil, fmt.Errorf("manager.NewWithPaths: inconsistent paths: %w", err)
		}
		if config.Systemd {
			return systemd.NewUnifiedManager(config, path)
		}
		return fs2.NewManager(config, path)
	}
	if config.Systemd {
		return systemd.NewLegacyManager(config, paths)
	}
	return fs.NewManager(config, paths)
}

// getUnifiedPath is an implementation detail of libcontainer.
// Historically, libcontainer.Create saves cgroup paths as per-subsystem path
// map (as returned by cm.GetPaths(""), but with v2 we only have one single
// unified path (with "" as a key).
//
// This function converts from that map to string (using "" as a key),
// and also checks that the map itself is sane.
func getunifiedPath(paths map[string]string) (string, error) {
	if len(paths) > 1 {
		return "", fmt.Errorf("expected a single path, got %+v", paths)
	}
	path := paths[""]
	// can be empty
	if path != "" {
		if filepath.Clean(path) != path || !filepath.IsAbs(path) {
			return "", fmt.Errorf("invalid path: %q", path)
		}
	}
	return path, nil
}
