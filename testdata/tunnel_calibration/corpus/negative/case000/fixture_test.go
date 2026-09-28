package fixture

import (
	"io"
	"testing"
)

func TestContractProbe(t *testing.T) {
	r := Wrap()
	p := make([]byte, 2)
	n, err := r.Read(p)
	if n != 2 || err != nil || string(p) != "ab" {
		t.Fatalf("first read: n=%d err=%v data=%q", n, err, p)
	}
	rest, err := io.ReadAll(r)
	if err != nil || string(rest) != "cdef" {
		t.Fatalf("retained suffix: %q %v", rest, err)
	}
	for size := 1; size <= 7; size++ {
		r = Wrap()
		n, err = r.Read(nil)
		if n != 0 || err != nil {
			t.Fatalf("zero read: %d %v", n, err)
		}
		var out []byte
		buf := make([]byte, size)
		for {
			n, err = r.Read(buf)
			out = append(out, buf[:n]...)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		if string(out) != "abcdef" {
			t.Fatalf("size%d: %q", size, out)
		}
		n, err = r.Read(buf)
		if n != 0 || err != io.EOF {
			t.Fatalf("exact EOF: %d %v", n, err)
		}
	}
}
