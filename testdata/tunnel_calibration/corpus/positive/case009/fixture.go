package fixture

type Broad interface {
	GetStepRun()
	GetStepRunByExecutionID()
	InsertStepRun()
	ListStepRunsByRun()
	SaveStepRun()
}
type Narrow interface {
	GetStepRun()
	SaveStepRun()
}
type Runner struct{ store Narrow }

func NewRunner(stepRunStore Broad) *Runner { return &Runner{stepRunStore} }
