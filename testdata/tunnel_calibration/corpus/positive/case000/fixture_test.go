package fixture

import (
	"io"
	"testing"
)

func TestContractProbe(t *testing.T) {
	r := Wrap()
	p := make([]byte, 2)
	n, err := r.Read(p)
	if n != 2 || err != io.ErrShortBuffer || string(p) != "ab" {
		t.Fatalf("first read: n=%d err=%v data=%q", n, err, p)
	}
	n, err = r.Read(p)
	if n != 0 || err != io.EOF {
		t.Fatalf("lost suffix probe: n=%d err=%v", n, err)
	}
	r = Wrap()
	n, err = r.Read(nil)
	if n != 0 || err != io.ErrShortBuffer {
		t.Fatalf("broken zero read changed: %d %v", n, err)
	}
}
