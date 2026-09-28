package fixture

type Store interface {
	GetJob()
	ClaimItems()
	CountActiveJobs()
	CreateJob()
	CreateJobIfBelowLimit()
	GetAttemptByExecution()
	GetItem()
	InsertAttempt()
	ListJobs()
	ReserveAttemptNumber()
	SaveItem()
	SaveJob()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.GetJob() }
