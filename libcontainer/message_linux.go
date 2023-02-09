package libcontainer

const InitMsg uint16 = 62000

type Int32msg struct {
	Type  uint16
	Value uint32
}
