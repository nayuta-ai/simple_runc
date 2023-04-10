package cgroups

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencontainers/runc/libcontainer/userns"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	CgroupProcesses   = "cgroup.procs"
	unifiedMountpoint = "/sys/fs/cgroup"
	hybridMountpoint  = "/sys/fs/cgroup/unified"
)

var (
	isUnifiedOnce sync.Once
	isUnified     bool
	isHybridOnce  sync.Once
	isHybrid      bool
)

// IsCgroup2UnifiedMode returns whether we are running in cgroup v2 unified mode. (WIP)
func IsCgroup2UnifiedMode() bool {
	isUnifiedOnce.Do(func() {
		var st unix.Statfs_t
		err := unix.Statfs(unifiedMountpoint, &st)
		if err != nil {
			if os.IsNotExist(err) && userns.RunningInUserNS() {
				// ignore the "not found" error if running in userns
				logrus.WithError(err).Debugf("%s missing, assuming cgroup v1", unifiedMountpoint)
				isUnified = false
				return
			}
			panic(fmt.Sprintf("cannot statfs cgroup root: %s", err))
		}
		isUnified = st.Type == unix.CGROUP2_SUPER_MAGIC
	})
	return isUnified
}

// IsCgroup2HybridMode returns whether we are running in cgroup v2 hybrid mode.
func IsCgroup2HybridMode() bool {
	isHybridOnce.Do(func() {
		var st unix.Statfs_t
		err := unix.Statfs(hybridMountpoint, &st)
		if err != nil {
			isHybrid = false
			if !os.IsNotExist(err) {
				// Report unexpected errors.
				logrus.WithError(err).Debugf("statfs(%q) failed", hybridMountpoint)
			}
			return
		}
		isHybrid = st.Type == unix.CGROUP2_SUPER_MAGIC
	})
	return isHybrid
}

// ParseCgroupFile parses the given cgroup file, typically /proc/self/cgroup
// or /proc/<pid>/cgroup, into a map of subsystems to cgroup paths, e.g.
//
//	"cpu": "/user.slice/user-1000.slice"
//	"pids": "/user.slice/user-1000.slice"
//
// etc.
//
// Note that for cgroup v2 unified hierarchy, there are no per-controller
// cgroup paths, so the resulting map will have a single element where the key
// is empty string ("") and the value is the cgroup path the <pid> is in.
func ParseCgroupFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseCgroupFromReader(f)
}

func parseCgroupFromReader(r io.Reader) (map[string]string, error) {
	s := bufio.NewScanner(r)
	cgroups := make(map[string]string)

	for s.Scan() {
		text := s.Text()
		// from cgroups(7):
		// /proc/[pid]/cgroup
		// ...
		// For each cgroup hierarchy ... there is one entry
		// containing three colon-separated fields of the form:
		//     hierarchy-ID:subsystem-list:cgroup-path
		parts := strings.SplitN(text, ":", 3)
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid cgroup entry: must contain at least two colons: %v", text)
		}

		for _, subs := range strings.Split(parts[1], ",") {
			cgroups[subs] = parts[2]
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return cgroups, nil
}

// WriteCgroupProc writes the specified pid into the cgroup's cgroup.procs file
func WriteCgroupProc(dir string, pid int) error {
	// Normally dir should not be empty, one case is that cgroup subsystem
	// is not mounted, we will get empty dir, and we want it fail here.
	if dir == "" {
		return fmt.Errorf("no such directory for %s", CgroupProcesses)
	}

	// Dont attach any pid to the cgroup if -1 is specified as a pid
	if pid == -1 {
		return nil
	}

	file, err := OpenFile(dir, CgroupProcesses, os.O_WRONLY)
	if err != nil {
		return fmt.Errorf("failed to write %v: %w", pid, err)
	}
	defer file.Close()

	for i := 0; i < 5; i++ {
		_, err = file.WriteString(strconv.Itoa(pid))
		if err == nil {
			return nil
		}

		// EINVAL might mean that the task being added to cgroup.procs is in state
		// TASK_NEW. We should attempt to do so again.
		if errors.Is(err, unix.EINVAL) {
			time.Sleep(30 * time.Millisecond)
			continue
		}

		return fmt.Errorf("failed to write %v: %w", pid, err)
	}
	return err
}
