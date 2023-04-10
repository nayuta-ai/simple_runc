package manager

import (
	"testing"
)

func TestGetUnifiedPath(t *testing.T) {
	path, err := getunifiedPath(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != "" {
		t.Errorf("Expected to have '' as path instead of %s", path)
	}
}
