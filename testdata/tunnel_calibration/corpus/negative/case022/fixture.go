package fixture

type Store interface{ GetRuleByID() }
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.GetRuleByID() }
