package libcontainer

import (
	"os/exec"
	"testing"
)

func TestStart(t *testing.T) {
	cmd := exec.Command("/proc/self/exec", "init")
	p := &initProcess{
		cmd: cmd,
	}
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
}
