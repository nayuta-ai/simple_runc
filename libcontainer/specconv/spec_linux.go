package specconv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"

	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/simple_runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

type CreateOpts struct {
	CgroupName       string
	NoNewKeyring     bool
	NoPivotRoot      bool
	Spec             *specs.Spec
	UseSystemdCgroup bool
}

// getwd is a wrapper similar to os.Getwd, except it always gets
// the value from the kernel, which guarantees the returned value
// to be absolute and clean.
func getwd() (wd string, err error) {
	for {
		// This function is a wrapper around the 'getcwd' system call,
		// which is used to retrieve the pathname of the current working directory.
		wd, err = unix.Getwd()
		//nolint:errorlint // unix errors are bare
		if err != unix.EINTR {
			break
		}
	}
	return wd, os.NewSyscallError("getwd", err)
}

// CreateLibcontainerConfig creates a new libcontainer configuration from a
// given specification and a cgroup name
func CreateLibcontainerConfig(opts *CreateOpts) (*configs.Config, error) {
	// runc's cwd will always be the bundle path
	// A bundle path is a directory that contains all the files and configuration required
	// to run a container using the Open Container Initiative (OCI) runtime specification.
	cwd, err := getwd()
	if err != nil {
		return nil, err
	}
	spec := opts.Spec
	if spec.Root == nil {
		return nil, errors.New("root must be specified")
	}
	rootfsPath := spec.Root.Path
	if !filepath.IsAbs(rootfsPath) {
		rootfsPath = filepath.Join(cwd, rootfsPath)
	}
	labels := []string{}
	for k, v := range spec.Annotations {
		labels = append(labels, k+"="+v)
	}
	config := &configs.Config{
		Rootfs:      rootfsPath,
		NoPivotRoot: opts.NoPivotRoot,
		Readonlyfs:  spec.Root.Readonly,
		Hostname:    spec.Hostname,
		Labels:      append(labels, "bundle="+cwd),
	}

	c, err := CreateCgroupConfig(opts, nil)
	if err != nil {
		return nil, err
	}
	config.Cgroups = c
	return config, nil
}

func CreateCgroupConfig(opts *CreateOpts, defaultDevs []*devices.Device) (*configs.Cgroup, error) {
	var (
		// myCgroupPath string

		spec             = opts.Spec
		useSystemdCgroup = opts.UseSystemdCgroup
		// name             = opts.CgroupName
	)

	c := &configs.Cgroup{
		Systemd: useSystemdCgroup,
	}

	if useSystemdCgroup {
		sp, err := initSystemdProps(spec)
		if err != nil {
			return nil, err
		}
		c.SystemdProps = sp
	}
	return c, nil
}

// checkPropertyName checks if systemd property name is valid. A valid name
// should consist of latin letters only, and have least 3 of them.
func checkPropertyName(s string) error {
	if len(s) < 3 {
		return errors.New("too short")
	}
	// Check ASCII characters rather than Unicode runes,
	// so we have to use indexes rather than range.
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			continue
		}
		return errors.New("contains non-alphabetic character")
	}
	return nil
}

// Some systemd properties are documented as having "Sec" suffix
// (e.g. TimeoutStopSec) but are expected to have "USec" suffix
// here, so let's provide conversion to improve compatibility.
func convertSecToUSec(value dbus.Variant) (dbus.Variant, error) {
	var sec uint64
	const M = 1000000
	vi := value.Value()
	switch value.Signature().String() {
	case "y":
		sec = uint64(vi.(byte)) * M
	case "n":
		sec = uint64(vi.(int16)) * M
	case "q":
		sec = uint64(vi.(uint16)) * M
	case "i":
		sec = uint64(vi.(int32)) * M
	case "u":
		sec = uint64(vi.(uint32)) * M
	case "x":
		sec = uint64(vi.(int64)) * M
	case "t":
		sec = vi.(uint64) * M
	case "d":
		sec = uint64(vi.(float64) * M)
	default:
		return value, errors.New("not a number")
	}
	return dbus.MakeVariant(sec), nil
}

// The purpose of this function seems to be to parse annotations in the spec parameter
// and convert them into properties for systemd on a Linux system. Specifically,
// it appears to parse properties with the "org.systemd.property." prefix,
// convert the values of properties that end with "Sec" to microsecond values,
// and add them to the systemd properties slice.
func initSystemdProps(spec *specs.Spec) ([]systemdDbus.Property, error) {
	// By using annotations with the "org.systemd.property." prefix in a container
	// or service's OCI specification, it is possible to provide systemd-specific configuration information,
	// such as resource limits or dependencies, to the systemd service manager.
	// The use of systemd properties in this way allows for more precise control
	// and management of containerized services and processes on Linux systems.
	const keyPrefix = "org.systemd.property."

	// The systemdDbus.Property struct represents a single systemd property,
	// and a slice of these structs can be used to set or get multiple properties at once.
	var sp []systemdDbus.Property

	for k, v := range spec.Annotations {
		name := strings.TrimPrefix(k, keyPrefix)
		if len(name) == len(k) { // prefix not there
			continue
		}
		if err := checkPropertyName(name); err != nil {
			return nil, fmt.Errorf("annotation %s name incorrect: %w", k, err)
		}
		value, err := dbus.ParseVariant(v, dbus.Signature{})
		if err != nil {
			return nil, fmt.Errorf("annotation %s=%s value parse error: %w", k, v, err)
		}
		// Check for Sec suffix.
		if trimName := strings.TrimSuffix(name, "Sec"); len(trimName) < len(name) {
			// Check for a lowercase ascii a-z just before Sec.
			if ch := trimName[len(trimName)-1]; ch >= 'a' && ch <= 'z' {
				// Convert from Sec to USec.
				name = trimName + "USec"
				value, err = convertSecToUSec(value)
				if err != nil {
					return nil, fmt.Errorf("annotation %s=%s value parse error: %w", k, v, err)
				}
			}
		}
		sp = append(sp, systemdDbus.Property{Name: name, Value: value})
	}

	return sp, nil
}
