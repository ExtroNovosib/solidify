package fixture

type Store interface{ RevokeRefreshTokensForSessionIDs() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.RevokeRefreshTokensForSessionIDs() }
