package clean

// These examples sit exactly on the default thresholds. They ensure that
// the corpus rejects only values that exceed a configured maximum.

type BoundaryA struct{}
type BoundaryB struct{}
type BoundaryC struct{}
type BoundaryD struct{}

func DispatchBoundary(value any) int {
	switch value.(type) {
	case BoundaryA:
		return 1
	case BoundaryB:
		return 2
	case BoundaryC:
		return 3
	case BoundaryD:
		return 4
	default:
		return 0
	}
}

func VisitBoundary(value any) {
	if _, ok := value.(BoundaryA); ok {
		return
	} else if _, ok := value.(BoundaryB); ok {
		return
	} else if _, ok := value.(BoundaryC); ok {
		return
	} else if _, ok := value.(BoundaryD); ok {
		return
	}
}

type ReadOperations interface {
	Read()
	Peek()
}

type WriteOperations interface {
	Write()
	Flush()
}

type BoundedOperations interface {
	ReadOperations
	WriteOperations
	Close()
}

type FocusedService struct{}

func (FocusedService) Create()  {}
func (FocusedService) Read()    {}
func (FocusedService) Update()  {}
func (FocusedService) Delete()  {}
func (FocusedService) Search()  {}
func (FocusedService) Export()  {}
func (FocusedService) Import()  {}
func (FocusedService) Enable()  {}
func (FocusedService) Disable() {}
func (FocusedService) Archive() {}

func FormatRecord(a, b, c, d, e string) string {
	return a + b + c + d + e
}
