package cgroups

import "testing"

func TestParseCgroups(t *testing.T) {
	cgroups, err := ParseCgroupFile("/proc/self/cgroup")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cgroups["cpu"]; !ok {
		t.Fatal("unexpected error")
	}
}

// func TestCgroup(t *testing.T) {
// 	t.Error(IsCgroup2UnifiedMode())
// 	t.Error(IsCgroup2HybridMode())
// }
