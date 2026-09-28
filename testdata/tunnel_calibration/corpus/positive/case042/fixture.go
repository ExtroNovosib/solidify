package fixture

import "example.com/tunnelcalibration/dep"

type EditEventNote struct{ Auth *dep.Service }

func (c *EditEventNote) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
