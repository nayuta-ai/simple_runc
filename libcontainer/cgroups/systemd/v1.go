package systemd

import (
	"strings"
	"sync"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/simple_runc/libcontainer/configs"
)

type LegacyManager struct {
	mu      sync.Mutex
	cgroups *configs.Cgroup
	paths   map[string]string
	dbus    *dbusConnManager
}

func NewLegacyManager(cg *configs.Cgroup, paths map[string]string) (*LegacyManager, error) {
	return &LegacyManager{
		cgroups: cg,
		paths:   paths,
		dbus:    newDbusConnManager(false),
	}, nil
}

func (m *LegacyManager) Apply(pid int) error {
	var (
		c          = m.cgroups
		unitName   = getUnitName(c)
		slice      = "system.slice"
		properties []systemdDbus.Property
	)
	m.mu.Lock()
	defer m.mu.Unlock()

	if c.Parent != "" {
		slice = c.Parent
	}

	properties = append(properties, systemdDbus.PropDescription("libcontainer container "+c.Name))

	if strings.HasSuffix(unitName, ".slice") {
		// If we create a slice, the parent is defined via a Wants=.
		properties = append(properties, systemdDbus.PropWants(slice))
	} else {
		// Otherwise it's a scope, which we put into a Slice=.
		properties = append(properties, systemdDbus.PropSlice(slice))
		// Assume scopes always support delegation (supported since systemd v218).
		properties = append(properties, newProp("Delegate", true))
	}

	// only add pid if its valid, -1 is used w/ general slice creation.
	if pid != -1 {
		properties = append(properties, newProp("PIDs", []uint32{uint32(pid)}))
	}

	// Always enable accounting, this gets us the same behaviour as the fs implementation,
	// plus the kernel has some problems with joining the memory cgroup at a later time.
	properties = append(properties,
		newProp("MemoryAccounting", true),
		newProp("CPUAccounting", true),
		newProp("BlockIOAccounting", true),
		newProp("TasksAccounting", true),
	)

	// Assume DefaultDependencies= will always work (the check for it was previously broken.)
	properties = append(properties,
		newProp("DefaultDependencies", false))

	properties = append(properties, c.SystemdProps...)

	if err := startUnit(m.dbus, unitName, properties); err != nil {
		return err
	}

	if err := m.joinCgroups(pid); err != nil {
		return err
	}
	return nil
}

func (m *LegacyManager) joinCgroups(pid int) error {
	return nil
}
