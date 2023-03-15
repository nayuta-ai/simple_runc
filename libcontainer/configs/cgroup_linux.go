package configs

import (
	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
)

type Cgroup struct {
	// Name specifies the name of the cgroup
	Name string `json:"name,omitempty"`

	// Parent specifies the name of parent of cgroup or slice
	Parent string `json:"parent,omitempty"`

	// Path specifies the path to cgroups that are created and/or joined by the container.
	// The path is assumed to be relative to the host system cgroup mountpoint.
	Path string `json:"path"`

	// ScopePrefix describes prefix for the scope name
	ScopePrefix string `json:"scope_prefix"`

	// Resources contains various cgroups settings to apply
	//*Resources

	// Systemd tells if systemd should be used to manage cgroups.
	Systemd bool

	// SystemdProps are any additional properties for systemd,
	// derived from org.systemd.property.xxx annotations.
	// Ignored unless systemd is used for managing cgroups.
	SystemdProps []systemdDbus.Property `json:"-"`

	// Rootless tells if rootless cgroups should be used.
	Rootless bool

	// The host UID that should own the cgroup, or nil to accept
	// the default ownership.  This should only be set when the
	// cgroupfs is to be mounted read/write.
	// Not all cgroup manager implementations support changing
	// the ownership.
	OwnerUID *int `json:"owner_uid,omitempty"`
}
