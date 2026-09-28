package analyzer

import (
	"strings"
	"testing"
)

func TestTunnelLSPDiscardedRead(t *testing.T) {
	for _, tc := range []struct {
		name, retain string
		want         int
	}{{"discarded", "", 1}, {"prefix-only", "audit:=pt[:n];_ =audit", 1}, {"retained", "c.pending=pt[n:]", 0}, {"not-stream", "", 0}, {"unrelated-read", "", 0}, {"helper", "", 1}} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package p
import("io";"bytes")
type C struct{base io.Reader; pending []byte}
func decode(b []byte)([]byte,error){return b,nil}
func(c *C)Read(p []byte)(int,error){b:=make([]byte,6);_,err:=io.ReadFull(c.base,b);if err!=nil{return 0,err};pt,err:=decode(b);if err!=nil{return 0,err};n:=copy(p,pt);` + tc.retain + `;if n<len(pt){return n,io.ErrShortBuffer};return n,nil}
func wrap()io.Reader{return &C{base:bytes.NewReader([]byte("abcdef"))}}
`
			if tc.name == "unrelated-read" {
				src = strings.Replace(src, "decode(b)", "decode([]byte(\"record\"))", 1)
			}
			if tc.name == "helper" {
				src = strings.Replace(src, "func(c *C)Read(p []byte)(int,error){", "func(c *C)Read(p []byte)(int,error){return c.read(p)}\nfunc(c *C)read(p []byte)(int,error){", 1)
			}
			if tc.name == "not-stream" {
				src = strings.Replace(src, "func wrap()io.Reader", "func wrap()*C", 1)
			}
			fset, files := parseSource(t, src)
			issues := CheckLSPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
			if len(issues) != tc.want {
				t.Fatalf("got %v, want %d", issues, tc.want)
			}
			if tc.want == 1 && (issues[0].Check != CheckLSPDiscardedRead || len(issues[0].Related) != 2) {
				t.Fatalf("missing contract evidence: %v", issues)
			}
		})
	}
}
func TestTunnelLSPNoopDeadline(t *testing.T) {
	for _, forward := range []bool{false, true} {
		t.Run(map[bool]string{false: "ignored", true: "forwarded"}[forward], func(t *testing.T) {
			src := `package p
import("net";"time")
type C struct{net.Conn}
func(c *C)SetDeadline(t time.Time)error{return nil}
func(c *C)SetReadDeadline(t time.Time)error{return nil}
func(c *C)SetWriteDeadline(t time.Time)error{return nil}
func wrap(base net.Conn)net.Conn{return &C{base}}
`
			if forward {
				for _, name := range []string{"SetDeadline", "SetReadDeadline", "SetWriteDeadline"} {
					src = strings.Replace(src, "func(c *C)"+name+"(t time.Time)error{return nil}", "func(c *C)"+name+"(t time.Time)error{return c.Conn."+name+"(t)}", 1)
				}
			}
			fset, files := parseSource(t, src)
			issues := CheckLSPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
			want := 3
			if forward {
				want = 0
			}
			if len(issues) != want {
				t.Fatalf("got %v want %d", issues, want)
			}
		})
	}
}
func TestTunnelISPConstructorRole(t *testing.T) {
	for _, tc := range []struct {
		name, parameter, body string
		want                  int
	}{
		{"direct", "a Wide", "return &C{a}", 1},
		{"variadic-alias", "a ...Wide", "var x Wide;if len(a)>0{x=a[0]};return &C{x}", 1},
		{"narrow", "a Narrow", "return &C{a}", 0},
		{"constructor-use", "a Wide", "a.Save();return &C{a}", 0},
		{"ambiguous", "a,b Wide", "x:=a;x=b;return &C{x}", 0},
		{"escape", "a Wide", "consume(a);return &C{a}", 0},
		{"reassigned-unknown", "a Wide", "x:=a;x=factory();return &C{x}", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `package p
type Narrow interface{Get()}
type Wide interface{Get();Save()}
type C struct{store Narrow}
func consume(Wide){}
func factory()Wide{return nil}
func NewC(` + tc.parameter + `)*C{` + tc.body + `}
`
			fset, files := parseSource(t, src)
			issues := checkISPConstructorRoles(fset, files, typeCheckSource(t, fset, files), nil)
			if len(issues) != tc.want {
				t.Fatalf("got %v want %d", issues, tc.want)
			}
			if tc.want == 1 && len(issues[0].Groups) != 2 {
				t.Fatalf("missing method sets: %v", issues)
			}
		})
	}
}

func TestTunnelLSPBodylessHelper(t *testing.T) {
	for _, body := range []string{"return readAssembly(p)", "pt,err:=recordAssembly();if err!=nil{return 0,err};n:=copy(p,pt);if n<len(pt){return n,io.ErrShortBuffer};return n,nil"} {
		fset, files := parseSource(t, `package p
 import "io"
 type C struct{}
 func readAssembly([]byte)(int,error)
 func recordAssembly()([]byte,error)
 func(c *C)Read(p []byte)(int,error){`+body+`}
 func wrap()io.Reader{return &C{}}`)
		issues := CheckLSPWithTypes(fset, files, typeCheckSource(t, fset, files), DefaultConfig(), nil)
		if len(issues) != 0 {
			t.Fatalf("unknown assembly flow claimed: %v", issues)
		}
	}
}
