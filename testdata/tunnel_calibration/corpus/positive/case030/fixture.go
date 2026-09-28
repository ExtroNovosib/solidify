package fixture

import "example.com/tunnelcalibration/dep"

type AddEventNote struct{ Auth *dep.Service }

func (c *AddEventNote) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
