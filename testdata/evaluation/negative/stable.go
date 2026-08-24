package negative

type BoundaryService struct {
	a, b, c, d, e, f, g, h, i, j int
}

func (*BoundaryService) A() {}
func (*BoundaryService) B() {}
func (*BoundaryService) C() {}
func (*BoundaryService) D() {}
func (*BoundaryService) E() {}
func (*BoundaryService) F() {}
func (*BoundaryService) G() {}
func (*BoundaryService) H() {}
func (*BoundaryService) I() {}
func (*BoundaryService) J() {}

func RegisterContact(name, email, phone, address, city string) {}
func UpdateContact(name, email, phone, address, city string)   {}

type VariantA struct{}
type VariantB struct{}
type VariantC struct{}
type VariantD struct{}

func Dispatch(value any) {
	switch value.(type) {
	case VariantA:
	case VariantB:
	case VariantC:
	case VariantD:
	}
}

type BoundedPort interface {
	A()
	B()
	C()
	D()
	E()
	F()
	G()
	H()
}

type Repository interface {
	Get() error
	Save() error
	Delete() error
}

type FullRepository struct{}

func (FullRepository) Get() error    { return nil }
func (FullRepository) Save() error   { return nil }
func (FullRepository) Delete() error { return nil }

func UseRepository(repository Repository) error {
	if err := repository.Get(); err != nil {
		return err
	}
	if err := repository.Save(); err != nil {
		return err
	}
	return repository.Delete()
}

type LocalClient struct{}
type LocalService struct{ client *LocalClient }
