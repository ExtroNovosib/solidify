package fixture

type Store interface {
	EnsureSettings()
	ListRules()
	TryClaimRuleUse()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.EnsureSettings(); c.store.ListRules() }
