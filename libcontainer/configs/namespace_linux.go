package configs

const (
	NEWNET    NamespaceType = "NEWNET"
	NEWPID    NamespaceType = "NEWPID"
	NEWNS     NamespaceType = "NEWNS"
	NEWUTS    NamespaceType = "NEWUTS"
	NEWIPC    NamespaceType = "NEWIPC"
	NEWUSER   NamespaceType = "NEWUSER"
	NEWCGROUP NamespaceType = "NEWCGROUP"
)

// Namespace defines configuration for each namespace.  It specifies an
// alternate path that is able to be joined via setns.
type Namespace struct {
	Type NamespaceType `json:"type"`
	Path string        `json:"path"`
}

func (n *Namespaces) index(t NamespaceType) int {
	for i, ns := range *n {
		if ns.Type == t {
			return i
		}
	}
	return -1
}

func (n *Namespaces) Contains(t NamespaceType) bool {
	return n.index(t) != -1
}
