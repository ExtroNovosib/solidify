package fixture

type Store interface {
	EnsureSettings()
	ListRules()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.EnsureSettings(); c.store.ListRules() }
