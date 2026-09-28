package fixture

import "example.com/tunnelcalibration/dep"

func NewEvaluateDue(evaluators *dep.Service) *dep.Service {
	if evaluators == nil {
		return nil
	}
	return evaluators
}
