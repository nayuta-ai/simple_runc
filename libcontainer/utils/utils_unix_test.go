package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"testing"
)

func TestNewSockPair(t *testing.T) {
	parent, child, err := NewSockPair("test")
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	defer child.Close()

	// Check unix.SOCK_STREAM works
	// parent -> child
	parentMessage := "Test message from parent"
	if _, err := parent.Write([]byte(parentMessage)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1024)
	len, err := child.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	childMessage := string(buf[:len])
	if parentMessage != childMessage {
		t.Errorf("Got: %s, but expected: %s", childMessage, parentMessage)
	}
	// child -> parent
	childMessage = "Test message from child"
	if _, err := child.Write([]byte(childMessage)); err != nil {
		t.Fatal(err)
	}
	buf = make([]byte, 1024)
	len, err = parent.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	parentMessage = string(buf[:len])
	if parentMessage != childMessage {
		t.Errorf("Got: %s, but expected: %s", parentMessage, childMessage)
	}

	// Check unix.SOCK_CLOEXEC works
	var out bytes.Buffer
	cmd := exec.Command("/bin/sh", "-c", "ls -l /proc/self/fd")
	cmd.Stdout = &out
	if err = cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out.Bytes(), []byte(fmt.Sprintf(" %d ->", child.Fd()))) || bytes.Contains(out.Bytes(), []byte(fmt.Sprintf(" %d ->", parent.Fd()))) {
		fmt.Printf("parent: %v, child: %v \n", parent.Fd(), child.Fd())
		fmt.Println(out.String())
		t.Error("Child socket file descriptor was not closed by exec")
	}
}
