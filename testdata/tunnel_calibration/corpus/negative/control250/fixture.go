package fixture

type StepConfig interface{ sealed() }
type A struct{}

func (A) sealed() {}

type Kind string

const (
	X Kind = "x"
	Y Kind = "y"
	Z Kind = "z"
)

func Decode(k Kind) (StepConfig, error) {
	switch k {
	case X:
		return A{}, nil
	case Y:
		return A{}, nil
	case Z:
		return A{}, nil
	default:
		return nil, nil
	}
}
