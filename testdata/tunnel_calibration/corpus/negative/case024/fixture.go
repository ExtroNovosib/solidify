package fixture

type Store interface {
	ListEnabledByType()
	ListRules()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ListEnabledByType(); c.store.ListRules() }
