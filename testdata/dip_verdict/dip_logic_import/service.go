package logiclayer

import "example.com/dipverdict/adapters/postgres"

type Service struct {
	repo *postgres.Client
}

func NewService() *Service {
	return &Service{repo: postgres.NewClient()}
}
