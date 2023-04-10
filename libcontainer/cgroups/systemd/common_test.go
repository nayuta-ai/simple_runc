package systemd

import "testing"

// If use systemd in Docker Container, it pass the below test.
func TestIsRunningSystemd(t *testing.T) {
	if !IsRunningSystemd() {
		t.Errorf("unexpected error: systemd shoud run")
	}
}
