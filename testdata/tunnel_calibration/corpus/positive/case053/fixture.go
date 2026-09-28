package fixture

import "example.com/tunnelcalibration/dep"

type EvaluateSignal struct{ Evaluators *dep.Service }

func (c *EvaluateSignal) Handle() error {
	if c.Evaluators == nil {
		return nil
	}
	return c.Evaluators.Handle()
}
