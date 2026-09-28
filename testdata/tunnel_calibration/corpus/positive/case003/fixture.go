package fixture

import (
	"net"
	"time"
)

type Stream struct{ net.Conn }

func (c *Stream) SetDeadline(t time.Time) error { return nil }
