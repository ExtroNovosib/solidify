package fixture

import "example.com/tunnelcalibration/dep"

type EvaluatorWorker struct{ due *dep.Service }

func (c *EvaluatorWorker) Handle() error {
	if c.due == nil {
		return nil
	}
	return c.due.Handle()
}
