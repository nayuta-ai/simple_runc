package fs

import (
	"sync"

	"github.com/simple_runc/libcontainer/cgroups"
	"github.com/simple_runc/libcontainer/configs"
)

type Manager struct {
	mu      sync.Mutex
	cgroups *configs.Cgroup
	paths   map[string]string
}

var subsystems = []subsystem{
	// &CpusetGroup{},
	// &DevicesGroup{},
	// &MemoryGroup{},
	// &CpuGroup{},
	// &CpuacctGroup{},
	// &PidsGroup{},
	// &BlkioGroup{},
	// &HugetlbGroup{},
	// &NetClsGroup{},
	// &NetPrioGroup{},
	// &PerfEventGroup{},
	// &FreezerGroup{},
	// &RdmaGroup{},
	&NameGroup{GroupName: "name=systemd", Join: true},
}

func init() {
	// If using cgroups-hybrid mode then add a "" controller indicating
	// it should join the cgroups v2.
	if cgroups.IsCgroup2HybridMode() {
		subsystems = append(subsystems, &NameGroup{GroupName: "", Join: true})
	}
}

type subsystem interface {
	// Name returns the name of the subsystem.
	Name() string
	// // GetStats fills in the stats for the subsystem.
	// GetStats(path string, stats *cgroups.Stats) error
	// Apply creates and joins a cgroup, adding pid into it. Some
	// subsystems use resources to pre-configure the cgroup parents
	// before creating or joining it.
	Apply(path string, r *configs.Resources, pid int) error
	// // Set sets the cgroup resources.
	// Set(path string, r *configs.Resources) error
}

func NewManager(cg *configs.Cgroup, paths map[string]string) (*Manager, error) {
	// // Some v1 controllers (cpu, cpuset, and devices) expect
	// // cgroups.Resources to not be nil in Apply.
	// if cg.Resources == nil {
	// 	return nil, errors.New("cgroup v1 manager needs configs.Resources to be set during manager creation")
	// }
	// if cg.Resources.Unified != nil {
	// 	return nil, cgroups.ErrV1NoUnified
	// }

	// if paths == nil {
	// 	var err error
	// 	paths, err = initPaths(cg)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }
	return &Manager{
		cgroups: cg,
		paths:   paths,
	}, nil
}

// isIgnorableError returns whether err is a permission error (in the loose
// sense of the word). This includes EROFS (which for an unprivileged user is
// basically a permission error) and EACCES (for similar reasons) as well as
// the normal EPERM.
func isIgnorableError(rootless bool, err error) bool {
	// // We do not ignore errors if we are root.
	// if !rootless {
	// 	return false
	// }
	// // Is it an ordinary EPERM?
	// if errors.Is(err, os.ErrPermission) {
	// 	return true
	// }
	// // Handle some specific syscall errors.
	// var errno unix.Errno
	// if errors.As(err, &errno) {
	// 	return errno == unix.EROFS || errno == unix.EPERM || errno == unix.EACCES
	// }
	return false
}

func (m *Manager) Apply(pid int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c := m.cgroups

	for _, sys := range subsystems {
		name := sys.Name()
		p, ok := m.paths[name]
		if !ok {
			continue
		}
		if err := sys.Apply(p, c.Resources, pid); err != nil {
			// In the case of rootless (including euid=0 in userns), where an
			// explicit cgroup path hasn't been set, we don't bail on error in
			// case of permission problems here, but do delete the path from
			// the m.paths map, since it is either non-existent and could not
			// be created, or the pid could not be added to it.
			//
			// Cases where limits for the subsystem have been set are handled
			// later by Set, which fails with a friendly error (see
			// if path == "" in Set).
			if isIgnorableError(c.Rootless, err) && c.Path == "" {
				delete(m.paths, name)
				continue
			}
			return err
		}
	}
	return nil
}
