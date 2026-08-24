package positive

import "errors"

type LargeService struct {
	a int
	b string
	c bool
	d float64
	e byte
	f rune
	g complex64
	h uint
	i uintptr
	j []byte
	k map[string]string
}

func (service *LargeService) A() {
	switch service.a {
	case 0:
	case 1:
	case 2:
	case 3:
	case 4:
	case 5:
	case 6:
	case 7:
	case 8:
	case 9:
	case 10:
	case 11:
	case 12:
	case 13:
	case 14:
	case 15:
	case 16:
	case 17:
	case 18:
	case 19:
	case 20:
	case 21:
	case 22:
	case 23:
	case 24:
	case 25:
	case 26:
	case 27:
	case 28:
	case 29:
	case 30:
	case 31:
	case 32:
	case 33:
	case 34:
	case 35:
	case 36:
	case 37:
	case 38:
	case 39:
	case 40:
	case 41:
	case 42:
	case 43:
	case 44:
	case 45:
	case 46:
	case 47:
	case 48:
	case 49:
	}
}
func (*LargeService) B() {}
func (*LargeService) C() {}
func (*LargeService) D() {}
func (*LargeService) E() {}
func (*LargeService) F() {}
func (*LargeService) G() {}
func (*LargeService) H() {}
func (*LargeService) I() {}
func (*LargeService) J() {}
func (*LargeService) K() {}

func RegisterContact(name, email, phone, address, city, country, region, postal, company string) {}
func UpdateContact(name, email, phone, address, city, country, region, postal, company string)   {}

type VariantA struct{}
type VariantB struct{}
type VariantC struct{}
type VariantD struct{}
type VariantE struct{}

func Dispatch(value any) {
	switch value.(type) {
	case VariantA:
	case VariantB:
	case VariantC:
	case VariantD:
	case VariantE:
	}
}

type WidePort interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
	I()
}

type Repository interface {
	Get() error
	Save() error
	Delete() error
}

type ReadOnlyRepository struct{}

func (ReadOnlyRepository) Get() error    { return nil }
func (ReadOnlyRepository) Save() error   { return errors.ErrUnsupported }
func (ReadOnlyRepository) Delete() error { return nil }

func UseRepository(repository Repository) error {
	return repository.Get()
}
