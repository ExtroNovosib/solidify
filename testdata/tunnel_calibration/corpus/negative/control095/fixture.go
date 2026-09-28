package fixture

import "example.com/tunnelcalibration/domain"

func Map(a *domain.Aggregate) map[string]string {
	return map[string]string{"name": a.Name(), "id": a.ID()}
}
