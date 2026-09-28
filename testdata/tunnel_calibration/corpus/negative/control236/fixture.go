package fixture

import "errors"

type Entity struct{}

func NewAttempt(kind string) (*Entity, error) {
	switch kind {
	case "a", "b", "c", "d", "e", "f":
	default:
		return nil, errors.New("invalid")
	}
	return &Entity{}, nil
}
