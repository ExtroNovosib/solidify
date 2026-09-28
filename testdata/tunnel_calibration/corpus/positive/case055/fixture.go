package fixture

import "example.com/tunnelcalibration/dep"

type EvaluateDue struct{ Evaluators *dep.Service }

func (c *EvaluateDue) Handle() error {
	if c.Evaluators == nil {
		return nil
	}
	return c.Evaluators.Handle()
}
