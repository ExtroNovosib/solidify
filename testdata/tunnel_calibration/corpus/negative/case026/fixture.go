package fixture

type Store interface{ ClaimDue() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ClaimDue() }
