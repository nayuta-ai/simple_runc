package cgroups

import (
	"errors"
	"os"
	"testing"
)

func TestOpenat2(t *testing.T) {
	if IsCgroup2UnifiedMode() {
		t.Skip("test requires cgroup v1")
	}

	// Make sure we test openat2, not its fallback.
	openFallback = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
		return nil, errors.New("fallback")
	}
	defer func() { openFallback = openAndCheck }()

	for _, tc := range []struct{ dir, file string }{
		{"/sys/fs/cgroup/unified", "cgroup.procs"},
	} {
		fd, err := OpenFile(tc.dir, tc.file, os.O_WRONLY)
		if err != nil {
			t.Errorf("case %+v: %v", tc, err)
		}
		fd.Close()
	}
}
