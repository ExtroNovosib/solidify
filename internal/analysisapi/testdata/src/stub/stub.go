package stub

type Wide interface {
	A()
	B()
	C()
	D()
	E()
	F()
}

type implementation struct{}

func (implementation) A() { panic("unsupported") } // want "split the interface so implementers only declare what they support"
func (implementation) B() {}
func (implementation) C() {}
func (implementation) D() {}
func (implementation) E() {}
func (implementation) F() {}
