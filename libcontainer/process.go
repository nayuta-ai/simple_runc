package libcontainer

import (
	"io"
	"os"
)

type Process struct {
	Init       bool
	Stdin      io.Reader
	Stdout     io.Writer
	Stderr     io.Writer
	ExtraFiles []*os.File
}
