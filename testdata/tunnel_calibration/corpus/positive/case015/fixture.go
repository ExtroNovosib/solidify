package fixture

type Store interface {
	Counts()
	ClaimBatch()
	Complete()
	EnqueueForTunnel()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.Counts() }
