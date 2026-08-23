package wiringroot

import "net"

type Alpha struct{}
type Beta struct{}
type Gamma struct{}
type Delta struct{}
type Epsilon struct{}
type Zeta struct{}

type WireRoot struct {
	alpha   *Alpha
	beta    *Beta
	gamma   *Gamma
	delta   *Delta
	epsilon *Epsilon
	zeta    *Zeta
	conn    *net.UDPConn
	addr    *net.UDPAddr
}

func NewWireRoot(a *Alpha, b *Beta, c *Gamma, d *Delta, e *Epsilon, z *Zeta, conn *net.UDPConn, addr *net.UDPAddr) *WireRoot {
	return &WireRoot{alpha: a, beta: b, gamma: c, delta: d, epsilon: e, zeta: z, conn: conn, addr: addr}
}
