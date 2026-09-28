package fixture

type Store interface {
	ClaimDueEvaluations()
	UpsertDueEvaluation()
	AlreadyEvaluated()
	AppendSignal()
	ListSignalHistory()
	MarkEvaluated()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.ClaimDueEvaluations(); c.store.UpsertDueEvaluation() }
