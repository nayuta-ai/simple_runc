package specconv

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/godbus/dbus/v5"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	libcontainerUtils "github.com/opencontainers/runc/libcontainer/utils"
	"github.com/sirupsen/logrus"

	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/simple_runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

// AllowedDevices is the set of devices which are automatically included for
// all containers.
//
// # XXX (cyphar)
//
// This behaviour is at the very least "questionable" (if not outright
// wrong) according to the runtime-spec.
//
// Yes, we have to include certain devices other than the ones the user
// specifies, but several devices listed here are not part of the spec
// (including "mknod for any device"?!). In addition, these rules are
// appended to the user-provided set which means that users *cannot disable
// this behaviour*.
//
// ... unfortunately I'm too scared to change this now because who knows how
// many people depend on this (incorrect and arguably insecure) behaviour.
var AllowedDevices = []*devices.Device{
	// allow mknod for any device
	{
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       devices.Wildcard,
			Minor:       devices.Wildcard,
			Permissions: "m",
			Allow:       true,
		},
	},
	{
		Rule: devices.Rule{
			Type:        devices.BlockDevice,
			Major:       devices.Wildcard,
			Minor:       devices.Wildcard,
			Permissions: "m",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/null",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       3,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/random",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       8,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/full",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       7,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/tty",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       5,
			Minor:       0,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/zero",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       5,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/urandom",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       9,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	// /dev/pts/ - pts namespaces are "coming soon"
	{
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       136,
			Minor:       devices.Wildcard,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       5,
			Minor:       2,
			Permissions: "rwm",
			Allow:       true,
		},
	},
}

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

	defaultDevs, err := createDevices(spec, config)
	if err != nil {
		return nil, err
	}

	c, err := CreateCgroupConfig(opts, defaultDevs)
	if err != nil {
		return nil, err
	}
	config.Cgroups = c
	return config, nil
}

func CreateCgroupConfig(opts *CreateOpts, defaultDevs []*devices.Device) (*configs.Cgroup, error) {
	var (
		myCgroupPath string

		spec             = opts.Spec
		useSystemdCgroup = opts.UseSystemdCgroup
		name             = opts.CgroupName
	)

	c := &configs.Cgroup{
		Systemd:   useSystemdCgroup,
		Resources: &configs.Resources{},
	}

	if useSystemdCgroup {
		sp, err := initSystemdProps(spec)
		if err != nil {
			return nil, err
		}
		c.SystemdProps = sp
	}

	if spec.Linux != nil && spec.Linux.CgroupsPath != "" {
		if useSystemdCgroup {
			myCgroupPath = spec.Linux.CgroupsPath
		} else {
			myCgroupPath = libcontainerUtils.CleanPath(spec.Linux.CgroupsPath)
		}
	}

	if useSystemdCgroup {
		if myCgroupPath == "" {
			// Default for c.Parent is set by systemd cgroup drivers.
			c.ScopePrefix = "runc"
			c.Name = name
		} else {
			// Parse the path from expected "slice:prefix:name"
			// for e.g. "system.slice:docker:1234"
			parts := strings.Split(myCgroupPath, ":")
			if len(parts) != 3 {
				return nil, fmt.Errorf("expected cgroupsPath to be of format \"slice:prefix:name\" for systemd cgroups, got %q instead", myCgroupPath)
			}
			c.Parent = parts[0]
			c.ScopePrefix = parts[1]
			c.Name = parts[2]
		}
	} else {
		if myCgroupPath == "" {
			c.Name = name
		}
		c.Path = myCgroupPath
	}
	// In rootless containers, any attempt to make cgroup changes is likely to fail.
	// libcontainer will validate this but ignores the error.
	if spec.Linux != nil {
		r := spec.Linux.Resources
		if r != nil {
			for i, d := range r.Devices {
				var (
					t     = "a"
					major = int64(-1)
					minor = int64(-1)
				)
				if d.Type != "" {
					t = d.Type
				}
				if d.Major != nil {
					major = *d.Major
				}
				if d.Minor != nil {
					minor = *d.Minor
				}
				if d.Access == "" {
					return nil, fmt.Errorf("device access at %d field cannot be empty", i)
				}
				dt, err := stringToCgroupDeviceRune(t)
				if err != nil {
					return nil, err
				}
				c.Resources.Devices = append(c.Resources.Devices, &devices.Rule{
					Type:        dt,
					Major:       major,
					Minor:       minor,
					Permissions: devices.Permissions(d.Access),
					Allow:       d.Allow,
				})
			}
			if r.Memory != nil {
				if r.Memory.Limit != nil {
					c.Resources.Memory = *r.Memory.Limit
				}
				if r.Memory.Reservation != nil {
					c.Resources.MemoryReservation = *r.Memory.Reservation
				}
				if r.Memory.Swap != nil {
					c.Resources.MemorySwap = *r.Memory.Swap
				}
				if r.Memory.Kernel != nil || r.Memory.KernelTCP != nil {
					logrus.Warn("Kernel memory settings are ignored and will be removed")
				}
				if r.Memory.Swappiness != nil {
					c.Resources.MemorySwappiness = r.Memory.Swappiness
				}
				if r.Memory.DisableOOMKiller != nil {
					c.Resources.OomKillDisable = *r.Memory.DisableOOMKiller
				}
				if r.Memory.CheckBeforeUpdate != nil {
					c.Resources.MemoryCheckBeforeUpdate = *r.Memory.CheckBeforeUpdate
				}
			}
			if r.CPU != nil {
				if r.CPU.Shares != nil {
					c.Resources.CpuShares = *r.CPU.Shares

					// CpuWeight is used for cgroupv2 and should be converted
					c.Resources.CpuWeight = cgroups.ConvertCPUSharesToCgroupV2Value(c.Resources.CpuShares)
				}
				if r.CPU.Quota != nil {
					c.Resources.CpuQuota = *r.CPU.Quota
				}
				if r.CPU.Period != nil {
					c.Resources.CpuPeriod = *r.CPU.Period
				}
				if r.CPU.RealtimeRuntime != nil {
					c.Resources.CpuRtRuntime = *r.CPU.RealtimeRuntime
				}
				if r.CPU.RealtimePeriod != nil {
					c.Resources.CpuRtPeriod = *r.CPU.RealtimePeriod
				}
				c.Resources.CpusetCpus = r.CPU.Cpus
				c.Resources.CpusetMems = r.CPU.Mems
				c.Resources.CPUIdle = r.CPU.Idle
			}
			if r.Pids != nil {
				c.Resources.PidsLimit = r.Pids.Limit
			}
			if r.BlockIO != nil {
				if r.BlockIO.Weight != nil {
					c.Resources.BlkioWeight = *r.BlockIO.Weight
				}
				if r.BlockIO.LeafWeight != nil {
					c.Resources.BlkioLeafWeight = *r.BlockIO.LeafWeight
				}
				if r.BlockIO.WeightDevice != nil {
					for _, wd := range r.BlockIO.WeightDevice {
						var weight, leafWeight uint16
						if wd.Weight != nil {
							weight = *wd.Weight
						}
						if wd.LeafWeight != nil {
							leafWeight = *wd.LeafWeight
						}
						weightDevice := configs.NewWeightDevice(wd.Major, wd.Minor, weight, leafWeight)
						c.Resources.BlkioWeightDevice = append(c.Resources.BlkioWeightDevice, weightDevice)
					}
				}
				if r.BlockIO.ThrottleReadBpsDevice != nil {
					for _, td := range r.BlockIO.ThrottleReadBpsDevice {
						rate := td.Rate
						throttleDevice := configs.NewThrottleDevice(td.Major, td.Minor, rate)
						c.Resources.BlkioThrottleReadBpsDevice = append(c.Resources.BlkioThrottleReadBpsDevice, throttleDevice)
					}
				}
				if r.BlockIO.ThrottleWriteBpsDevice != nil {
					for _, td := range r.BlockIO.ThrottleWriteBpsDevice {
						rate := td.Rate
						throttleDevice := configs.NewThrottleDevice(td.Major, td.Minor, rate)
						c.Resources.BlkioThrottleWriteBpsDevice = append(c.Resources.BlkioThrottleWriteBpsDevice, throttleDevice)
					}
				}
				if r.BlockIO.ThrottleReadIOPSDevice != nil {
					for _, td := range r.BlockIO.ThrottleReadIOPSDevice {
						rate := td.Rate
						throttleDevice := configs.NewThrottleDevice(td.Major, td.Minor, rate)
						c.Resources.BlkioThrottleReadIOPSDevice = append(c.Resources.BlkioThrottleReadIOPSDevice, throttleDevice)
					}
				}
				if r.BlockIO.ThrottleWriteIOPSDevice != nil {
					for _, td := range r.BlockIO.ThrottleWriteIOPSDevice {
						rate := td.Rate
						throttleDevice := configs.NewThrottleDevice(td.Major, td.Minor, rate)
						c.Resources.BlkioThrottleWriteIOPSDevice = append(c.Resources.BlkioThrottleWriteIOPSDevice, throttleDevice)
					}
				}
			}
			for _, l := range r.HugepageLimits {
				c.Resources.HugetlbLimit = append(c.Resources.HugetlbLimit, &configs.HugepageLimit{
					Pagesize: l.Pagesize,
					Limit:    l.Limit,
				})
			}
			if len(r.Rdma) > 0 {
				c.Resources.Rdma = make(map[string]configs.LinuxRdma, len(r.Rdma))
				for k, v := range r.Rdma {
					c.Resources.Rdma[k] = configs.LinuxRdma{
						HcaHandles: v.HcaHandles,
						HcaObjects: v.HcaObjects,
					}
				}
			}
			if r.Network != nil {
				if r.Network.ClassID != nil {
					c.Resources.NetClsClassid = *r.Network.ClassID
				}
				for _, m := range r.Network.Priorities {
					c.Resources.NetPrioIfpriomap = append(c.Resources.NetPrioIfpriomap, &configs.IfPrioMap{
						Interface: m.Name,
						Priority:  int64(m.Priority),
					})
				}
			}
			if len(r.Unified) > 0 {
				// copy the map
				c.Resources.Unified = make(map[string]string, len(r.Unified))
				for k, v := range r.Unified {
					c.Resources.Unified[k] = v
				}
			}
		}
	}
	// Append the default allowed devices to the end of the list.
	for _, device := range defaultDevs {
		c.Resources.Devices = append(c.Resources.Devices, &device.Rule)
	}
	return c, nil
}

func stringToCgroupDeviceRune(s string) (devices.Type, error) {
	switch s {
	case "a":
		return devices.WildcardDevice, nil
	case "b":
		return devices.BlockDevice, nil
	case "c":
		return devices.CharDevice, nil
	default:
		return 0, fmt.Errorf("invalid cgroup device type %q", s)
	}
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

func stringToDeviceRune(s string) (devices.Type, error) {
	switch s {
	case "p":
		return devices.FifoDevice, nil
	case "u", "c":
		return devices.CharDevice, nil
	case "b":
		return devices.BlockDevice, nil
	default:
		return 0, fmt.Errorf("invalid device type %q", s)
	}
}

func createDevices(spec *specs.Spec, config *configs.Config) ([]*devices.Device, error) {
	// If a spec device is redundant with a default device, remove that default
	// device (the spec one takes priority).
	dedupedAllowDevs := []*devices.Device{}

next:
	for _, ad := range AllowedDevices {
		if ad.Path != "" && spec.Linux != nil {
			for _, sd := range spec.Linux.Devices {
				if sd.Path == ad.Path {
					continue next
				}
			}
		}
		dedupedAllowDevs = append(dedupedAllowDevs, ad)
		if ad.Path != "" {
			config.Devices = append(config.Devices, ad)
		}
	}

	// Merge in additional devices from the spec.
	if spec.Linux != nil {
		for _, d := range spec.Linux.Devices {
			var uid, gid uint32
			var filemode os.FileMode = 0o666

			if d.UID != nil {
				uid = *d.UID
			}
			if d.GID != nil {
				gid = *d.GID
			}
			dt, err := stringToDeviceRune(d.Type)
			if err != nil {
				return nil, err
			}
			if d.FileMode != nil {
				filemode = *d.FileMode &^ unix.S_IFMT
			}
			device := &devices.Device{
				Rule: devices.Rule{
					Type:  dt,
					Major: d.Major,
					Minor: d.Minor,
				},
				Path:     d.Path,
				FileMode: filemode,
				Uid:      uid,
				Gid:      gid,
			}
			config.Devices = append(config.Devices, device)
		}
	}

	return dedupedAllowDevs, nil
}
