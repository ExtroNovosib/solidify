package fixture

type Store interface {
	ListEnabledByType()
	ListRules()
	CountRules()
	CreateRule()
	DeleteRule()
	GetRule()
	GetRuleByID()
	UpdateRule()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ListEnabledByType(); c.store.ListRules() }
