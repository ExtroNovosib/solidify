package fixture

type Broad interface {
	AdvanceRunStep()
	CancelRun()
	CreateRunWithFirstStep()
	CreateTriggeredRun()
	FailExpiredRuns()
	FinishRun()
	GetRun()
	ListRunsByOwner()
	ListRunsByWorkflow()
	StartRun()
}
type Narrow interface {
	AdvanceRunStep()
	FinishRun()
	GetRun()
	StartRun()
}
type Runner struct{ store Narrow }

func NewRunner(runStore Narrow) *Runner { return &Runner{runStore} }
