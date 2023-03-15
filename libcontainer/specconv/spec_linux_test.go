package specconv

import (
	"testing"

	"github.com/simple_runc/libcontainer/configs/validate"
)

func TestGetWd(t *testing.T) {
	_, err := getwd()
	if err != nil {
		t.Fatal("")
	}
}

func TestCreateLibcontainerConfig(t *testing.T) {
	spec := Example()
	spec.Root.Path = "/"
	opts := &CreateOpts{
		Spec:       spec,
		CgroupName: "Container ID",
	}
	config, err := CreateLibcontainerConfig(opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := validate.Validate(config); err != nil {
		t.Errorf("Expected specconv to produce valid container config: %v", err)
	}
}
