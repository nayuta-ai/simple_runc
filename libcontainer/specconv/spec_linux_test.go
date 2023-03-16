package specconv

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/simple_runc/libcontainer/configs"
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

func TestCheckPropertyName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected error
	}{
		{
			name:     "valid property name",
			input:    "validName",
			expected: nil,
		},
		{
			name:     "too short name",
			input:    "aa",
			expected: errors.New("too short"),
		},
		{
			name:     "contains non-alphabetic character",
			input:    "invalid_name!",
			expected: errors.New("contains non-alphabetic character"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := checkPropertyName(tt.input)
			if tt.expected == nil && actual != nil || tt.expected != nil && actual == nil {
				t.Errorf("unexpected error: got %v, want %v", actual, tt.expected)
			} else if tt.expected != nil && actual != nil && tt.expected.Error() != actual.Error() {
				t.Errorf("unexpected error: got %v, want %v", actual, tt.expected)
			}
		})
	}
}

func TestConvertSecToUSec(t *testing.T) {
	tests := []struct {
		name     string
		input    dbus.Variant
		expected dbus.Variant
		wantErr  bool
	}{
		{
			name:     "convert byte to uint64",
			input:    dbus.MakeVariant(byte(2)),
			expected: dbus.MakeVariant(uint64(2000000)),
			wantErr:  false,
		},
		{
			name:     "convert int16 to uint64",
			input:    dbus.MakeVariant(int16(3)),
			expected: dbus.MakeVariant(uint64(3000000)),
			wantErr:  false,
		},
		{
			name:     "convert uint16 to uint64",
			input:    dbus.MakeVariant(uint16(4)),
			expected: dbus.MakeVariant(uint64(4000000)),
			wantErr:  false,
		},
		{
			name:     "convert int32 to uint64",
			input:    dbus.MakeVariant(int32(5)),
			expected: dbus.MakeVariant(uint64(5000000)),
			wantErr:  false,
		},
		{
			name:     "convert uint32 to uint64",
			input:    dbus.MakeVariant(uint32(6)),
			expected: dbus.MakeVariant(uint64(6000000)),
			wantErr:  false,
		},
		{
			name:     "convert int64 to uint64",
			input:    dbus.MakeVariant(int64(7)),
			expected: dbus.MakeVariant(uint64(7000000)),
			wantErr:  false,
		},
		{
			name:     "convert uint64 to uint64",
			input:    dbus.MakeVariant(uint64(8)),
			expected: dbus.MakeVariant(uint64(8000000)),
			wantErr:  false,
		},
		{
			name:     "convert float64 to uint64",
			input:    dbus.MakeVariant(float64(9.5)),
			expected: dbus.MakeVariant(uint64(9500000)),
			wantErr:  false,
		},
		{
			name:     "unsupported type",
			input:    dbus.MakeVariant("unsupported"),
			expected: dbus.MakeVariant("unsupported"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := convertSecToUSec(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("unexpected error: got %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && actual.Value() != tt.expected.Value() {
				t.Errorf("unexpected value: got %v, want %v", actual.Value(), tt.expected.Value())
			}
		})
	}
}

func TestInitSystemdProps(t *testing.T) {
	spec := &specs.Spec{
		Annotations: map[string]string{
			"org.systemd.property.TestSec": "1 sec",
		},
	}

	expectedProps := []systemdDbus.Property{
		{Name: "TestUSec", Value: dbus.MakeVariant(uint64(1000000))},
	}

	props, err := initSystemdProps(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(props) != len(expectedProps) {
		t.Fatalf("unexpected number of properties: got %d, want %d", len(props), len(expectedProps))
	}

	for i, prop := range props {
		if prop.Name != expectedProps[i].Name {
			t.Errorf("unexpected property name: got %s, want %s", prop.Name, expectedProps[i].Name)
		}

		if !reflect.DeepEqual(prop.Value, expectedProps[i].Value) {
			t.Errorf("unexpected property value: got %v, want %v", prop.Value, expectedProps[i].Value)
		}
	}
}

func TestStringToCgroupDeviceRune(t *testing.T) {
	dt, _ := stringToCgroupDeviceRune("a")
	if dt != devices.WildcardDevice {
		t.Errorf("Expected to have %d as Name instead of %d", dt, devices.WildcardDevice)
	}
	dt, _ = stringToCgroupDeviceRune("b")
	if dt != devices.BlockDevice {
		t.Errorf("Expected to have %d as Name instead of %d", dt, devices.BlockDevice)
	}
	dt, _ = stringToCgroupDeviceRune("c")
	if dt != devices.CharDevice {
		t.Errorf("Expected to have %d as Name instead of %d", dt, devices.CharDevice)
	}
	_, err := stringToCgroupDeviceRune("d")
	expectedErr := fmt.Errorf("invalid cgroup device type d")
	if errors.Is(err, expectedErr) {
		t.Errorf("Expected to have %q as Name instead of %d", err, expectedErr)
	}
}

func TestStringToDeviceRune(t *testing.T) {
	dt, _ := stringToDeviceRune("p")
	if dt != devices.FifoDevice {
		t.Errorf("Expected to have %d as Name instead of %d", dt, devices.FifoDevice)
	}
	dt, _ = stringToDeviceRune("u")
	if dt != devices.CharDevice {
		t.Errorf("Expected to have %d as Name instead of %d", dt, devices.CharDevice)
	}
	dt, _ = stringToDeviceRune("c")
	if dt != devices.CharDevice {
		t.Errorf("Expected to have %d as Name instead of %d", dt, devices.CharDevice)
	}
	dt, _ = stringToDeviceRune("b")
	if dt != devices.BlockDevice {
		t.Errorf("Expected to have %d as Name instead of %d", dt, devices.BlockDevice)
	}
	_, err := stringToDeviceRune("d")
	expectedErr := fmt.Errorf("invalid device type d")
	if errors.Is(err, expectedErr) {
		t.Errorf("Expected to have %q as Name instead of %d", err, expectedErr)
	}
}

// (WIP)
func TestLinuxCgroupWithResource(t *testing.T) {
	cgroupsPath := "/user/cgroups/path/id"

	spec := &specs.Spec{}
	devices := []specs.LinuxDeviceCgroup{
		{
			Allow:  false,
			Access: "rwm",
		},
	}

	limit := int64(100)
	reservation := int64(50)
	swap := int64(20)
	kernel := int64(40)
	kernelTCP := int64(45)
	swappiness := uint64(1)
	swappinessPtr := &swappiness
	disableOOMKiller := true
	shares := uint64(40)
	quota := int64(20)
	period := uint64(30)
	realtimeRuntime := int64(10)
	realtimePeriod := uint64(10)
	cpus := "cpu"
	mems := "mem"
	idle := int64(10)
	pidsLimit := int64(100)
	resources := &specs.LinuxResources{
		Devices: devices,
		Memory: &specs.LinuxMemory{
			Limit:            &limit,
			Reservation:      &reservation,
			Swap:             &swap,
			Kernel:           &kernel,
			KernelTCP:        &kernelTCP,
			Swappiness:       swappinessPtr,
			DisableOOMKiller: &disableOOMKiller,
		},
		CPU: &specs.LinuxCPU{
			Shares:          &shares,
			Quota:           &quota,
			Period:          &period,
			RealtimeRuntime: &realtimeRuntime,
			RealtimePeriod:  &realtimePeriod,
			Cpus:            cpus,
			Mems:            mems,
			Idle:            &idle,
		},
		Pids: &specs.LinuxPids{
			Limit: pidsLimit,
		},
		// BlockIO
		// HugepageLimits
		// Rdma
		// Network
		// Unified
	}
	spec.Linux = &specs.Linux{
		CgroupsPath: cgroupsPath,
		Resources:   resources,
	}

	opts := &CreateOpts{
		CgroupName:       "ContainerID",
		UseSystemdCgroup: false,
		Spec:             spec,
	}

	cgroup, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	if cgroup.Path != cgroupsPath {
		t.Errorf("Wrong cgroupsPath, expected '%s' got '%s'", cgroupsPath, cgroup.Path)
	}
	if cgroup.Resources.Memory != limit {
		t.Errorf("Expected to have %d as memory limit, got %d", limit, cgroup.Resources.Memory)
	}
	if cgroup.Resources.MemoryReservation != reservation {
		t.Errorf("Expected to have %d as memory reservation, got %d", reservation, cgroup.Resources.MemoryReservation)
	}
	if cgroup.Resources.MemorySwap != swap {
		t.Errorf("Expected to have %d as swap, got %d", swap, cgroup.Resources.MemorySwap)
	}
	if cgroup.Resources.MemorySwappiness != swappinessPtr {
		t.Errorf("Expected to have %d as memory swappiness, got %d", swappinessPtr, cgroup.Resources.MemorySwappiness)
	}
	if cgroup.Resources.OomKillDisable != disableOOMKiller {
		t.Errorf("The OOMKiller should be enabled")
	}

	if cgroup.Resources.CpuShares != shares {
		t.Errorf("Expected to have %d as cpu shares, got %d", shares, cgroup.Resources.CpuShares)
	}
	if cgroup.Resources.CpuQuota != quota {
		t.Errorf("Expected to have %d as cpu quota, got %d", quota, cgroup.Resources.CpuQuota)
	}
	if cgroup.Resources.CpuPeriod != period {
		t.Errorf("Expected to have %d as cpu period, got %d", period, cgroup.Resources.CpuPeriod)
	}
	if cgroup.Resources.CpuRtRuntime != realtimeRuntime {
		t.Errorf("Expected to have %d as cpu realtime runtime, got %d", realtimeRuntime, cgroup.Resources.CpuRtRuntime)
	}
	if cgroup.Resources.CpuRtPeriod != realtimePeriod {
		t.Errorf("Expected to have %d as cpu realtime period, got %d", realtimePeriod, cgroup.Resources.CpuRtPeriod)
	}
	if cgroup.Resources.CpusetCpus != cpus {
		t.Errorf("Wrong cgroupsCpusetCpus, expected '%s' got '%s'", cpus, cgroup.Resources.CpusetCpus)
	}
	if cgroup.Resources.CpusetMems != mems {
		t.Errorf("Wrong cgroupsCpusetMems, expected '%s' got '%s'", mems, cgroup.Resources.CpusetMems)
	}
	if *cgroup.Resources.CPUIdle != idle {
		t.Errorf("Expected to have %d as cpu idle, got %d", idle, *cgroup.Resources.CPUIdle)
	}
	if cgroup.Resources.PidsLimit != pidsLimit {
		t.Errorf("Expected to have %d as memory limit, got %d", pidsLimit, cgroup.Resources.PidsLimit)
	}
}

func TestLinuxCgroupSystemd(t *testing.T) {
	cgroupsPath := "parent:scopeprefix:name"

	spec := &specs.Spec{
		Annotations: map[string]string{
			"org.systemd.property.TestSec": "1 sec",
		},
	}

	spec.Linux = &specs.Linux{
		CgroupsPath: cgroupsPath,
	}
	opts := &CreateOpts{
		Spec:             spec,
		UseSystemdCgroup: true,
	}
	cgroup, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedParent := "parent"
	if cgroup.Parent != expectedParent {
		t.Errorf("Expected to have %s as Parent instead of %s", expectedParent, cgroup.Parent)
	}

	expectedScopePrefix := "scopeprefix"
	if cgroup.ScopePrefix != expectedScopePrefix {
		t.Errorf("Expected to have %s as ScopePrefix instead of %s", expectedScopePrefix, cgroup.ScopePrefix)
	}

	expectedName := "name"
	if cgroup.Name != expectedName {
		t.Errorf("Expected to have %s as Name instead of %s", expectedName, cgroup.Name)
	}

	expectedProps := []systemdDbus.Property{
		{Name: "TestUSec", Value: dbus.MakeVariant(uint64(1000000))},
	}
	if len(cgroup.SystemdProps) != len(expectedProps) {
		t.Fatalf("unexpected number of properties: got %d, want %d", len(cgroup.SystemdProps), len(expectedProps))
	}
	for i, prop := range cgroup.SystemdProps {
		if prop.Name != expectedProps[i].Name {
			t.Errorf("unexpected property name: got %s, want %s", prop.Name, expectedProps[i].Name)
		}

		if !reflect.DeepEqual(prop.Value, expectedProps[i].Value) {
			t.Errorf("unexpected property value: got %v, want %v", prop.Value, expectedProps[i].Value)
		}
	}
}

func TestLinuxCgroupSystemdWithEmptyPath(t *testing.T) {
	cgroupsPath := ""

	spec := &specs.Spec{}
	spec.Linux = &specs.Linux{
		CgroupsPath: cgroupsPath,
	}
	opts := &CreateOpts{
		Spec:             spec,
		CgroupName:       "ContainerID",
		UseSystemdCgroup: true,
	}
	cgroup, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		t.Errorf("Couldn't create Cgroup config: %v", err)
	}

	expectedParent := ""
	if cgroup.Parent != expectedParent {
		t.Errorf("Expected to have %s as Parent instead of %s", expectedParent, cgroup.Parent)
	}

	expectedScopePrefix := "runc"
	if cgroup.ScopePrefix != expectedScopePrefix {
		t.Errorf("Expected to have %s as ScopePrefix instead of %s", expectedScopePrefix, cgroup.ScopePrefix)
	}

	if cgroup.Name != opts.CgroupName {
		t.Errorf("Expected to have %s as Name instead of %s", opts.CgroupName, cgroup.Name)
	}

	if len(cgroup.SystemdProps) > 0 {
		t.Errorf("Expected to have [] as SystemdProps instead of %v", cgroup.SystemdProps)
	}
}

func TestCreateDevice(t *testing.T) {
	spec := Example()
	// dummy uid/gid for /dev/tty; will enable the test to check if createDevices()
	// preferred the spec's device over the redundant default device
	ttyUid := uint32(1000)
	ttyGid := uint32(1000)
	fm := os.FileMode(0o666)

	spec.Linux = &specs.Linux{
		Devices: []specs.LinuxDevice{
			{
				// This is purposely redundant with one of runc's default devices
				Path:     "/dev/tty",
				Type:     "c",
				Major:    5,
				Minor:    0,
				FileMode: &fm,
				UID:      &ttyUid,
				GID:      &ttyGid,
			},
			{
				// This is purposely not redundant with one of runc's default devices
				Path:  "/dev/ram0",
				Type:  "b",
				Major: 1,
				Minor: 0,
			},
		},
	}

	conf := &configs.Config{}

	defaultDevs, err := createDevices(spec, conf)
	if err != nil {
		t.Errorf("failed to create devices: %v", err)
	}

	// Verify the returned default devices has the /dev/tty entry deduplicated
	found := false
	for _, d := range defaultDevs {
		if d.Path == "/dev/tty" {
			if found {
				t.Errorf("createDevices failed: returned a duplicated device entry: %v", defaultDevs)
			}
			found = true
		}
	}

	// Verify that createDevices() placed all default devices in the config
	for _, allowedDev := range AllowedDevices {
		if allowedDev.Path == "" {
			continue
		}

		found := false
		for _, configDev := range conf.Devices {
			if configDev.Path == allowedDev.Path {
				found = true
			}
		}
		if !found {
			configDevPaths := []string{}
			for _, configDev := range conf.Devices {
				configDevPaths = append(configDevPaths, configDev.Path)
			}
			t.Errorf("allowedDevice %s was not found in the config's devices: %v", allowedDev.Path, configDevPaths)
		}
	}

	// Verify that createDevices() deduplicated the /dev/tty entry in the config
	for _, configDev := range conf.Devices {
		if configDev.Path == "/dev/tty" {
			wantDev := &devices.Device{
				Path:     "/dev/tty",
				FileMode: 0o666,
				Uid:      1000,
				Gid:      1000,
				Rule: devices.Rule{
					Type:  devices.CharDevice,
					Major: 5,
					Minor: 0,
				},
			}

			if *configDev != *wantDev {
				t.Errorf("redundant dev was not deduplicated correctly: want %v, got %v", wantDev, configDev)
			}
		}
	}

	// Verify that createDevices() added the entry for /dev/ram0 in the config
	found = false
	for _, configDev := range conf.Devices {
		if configDev.Path == "/dev/ram0" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("device /dev/ram0 not found in config devices; got %v", conf.Devices)
	}
}
