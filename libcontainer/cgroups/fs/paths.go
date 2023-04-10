package fs

import (
	"os"

	"github.com/simple_runc/libcontainer/cgroups"
)

func apply(path string, pid int) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return cgroups.WriteCgroupProc(path, pid)
}
