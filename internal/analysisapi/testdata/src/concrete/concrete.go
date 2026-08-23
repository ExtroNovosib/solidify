package concrete

import "concrete/store"

type Service struct{}

func NewService(store *store.Repo) *Service { // want "depends on the concrete type"
	return &Service{}
}
