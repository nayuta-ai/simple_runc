package specconv

import (
	"errors"
	"reflect"
	"testing"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/opencontainers/runtime-spec/specs-go"
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
