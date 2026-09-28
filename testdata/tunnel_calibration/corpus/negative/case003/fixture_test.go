package fixture

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

func TestContractProbe(t *testing.T) {
	base, peer := net.Pipe()
	defer base.Close()
	defer peer.Close()
	c := &Stream{Conn: base}
	start := time.Now()
	if err := base.SetDeadline(start.Add(150 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDeadline(start.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err := c.Read(make([]byte, 1))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("deadline: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("expired deadline failed to interrupt promptly: %s", elapsed)
	}
}
