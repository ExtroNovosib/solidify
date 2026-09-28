package fixture

type Optional0 interface{ Capability0() }
type Optional1 interface{ Capability1() }
type Optional2 interface{ Capability2() }
type Optional3 interface{ Capability3() }
type Optional4 interface{ Capability4() }

func Inspect(s any) {
	if c, ok := s.(Optional0); ok {
		c.Capability0()
	}
	if c, ok := s.(Optional1); ok {
		c.Capability1()
	}
	if c, ok := s.(Optional2); ok {
		c.Capability2()
	}
	if c, ok := s.(Optional3); ok {
		c.Capability3()
	}
	if c, ok := s.(Optional4); ok {
		c.Capability4()
	}
}
