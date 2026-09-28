package fixture

type Store interface {
	GetRuleByID()
	CountRules()
	CreateRule()
	DeleteRule()
	GetRule()
	ListEnabledByType()
	ListRules()
	UpdateRule()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.GetRuleByID() }
