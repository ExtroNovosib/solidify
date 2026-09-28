package fixture

type Store interface {
	AppendSignal()
	UpsertDueEvaluation()
	AlreadyEvaluated()
	ClaimDueEvaluations()
	ListSignalHistory()
	MarkEvaluated()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.AppendSignal(); c.store.UpsertDueEvaluation() }
