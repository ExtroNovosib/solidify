package fixture

type Store interface{ ActiveEventGrant() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ActiveEventGrant() }
