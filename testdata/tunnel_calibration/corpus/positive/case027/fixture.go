package fixture

type Store interface {
	RevokeRefreshTokensForSessionIDs()
	Delete()
	Get()
	Save()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.RevokeRefreshTokensForSessionIDs() }
