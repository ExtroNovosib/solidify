package fixture

type Store interface {
	AppendSignal()
	UpsertDueEvaluation()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.AppendSignal(); c.store.UpsertDueEvaluation() }
