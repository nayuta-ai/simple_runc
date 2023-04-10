package manager

import (
	"testing"

	"github.com/simple_runc/libcontainer/configs"
)

func TestManager(t *testing.T) {
	for _, sd := range []bool{false, true} {
		cg := &configs.Cgroup{}
		cg.Systemd = sd
		mgr, err := New(cg)
		if err != nil {
			t.Error(err)
		}
		if mgr == nil {
			t.Error("unexpected error: mgr shouldn't NULL.")
		}
		_ = mgr.Apply(-1)
	}
}
