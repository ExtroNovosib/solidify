package fixture

type Store interface {
	ClaimDueEvaluations()
	UpsertDueEvaluation()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ClaimDueEvaluations(); c.store.UpsertDueEvaluation() }
