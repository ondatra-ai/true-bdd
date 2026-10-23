package shell_test

import (
	"context"
	"os"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/ondatra-ai/true-bdd/pkg/shell"
)

func TestSessionLeadsItsOwnSession(t *testing.T) {
	process, err := shell.Start(context.Background(), []string{"/bin/sleep", "10"}, shell.Options{Session: true})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = process.Signal(os.Kill); _, _ = process.Wait() })

	session, err := unix.Getsid(process.Pid())
	if err != nil {
		t.Fatal(err)
	}

	if session != process.Pid() {
		t.Fatalf("session %d != child %d", session, process.Pid())
	}
}
