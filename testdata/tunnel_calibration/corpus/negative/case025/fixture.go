package fixture

type Store interface{ ClaimItems() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ClaimItems() }
