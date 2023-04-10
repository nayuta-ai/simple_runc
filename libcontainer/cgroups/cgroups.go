package cgroups

type Manager interface {
	Apply(pid int) error
}
