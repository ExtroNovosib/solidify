package fixture

import "example.com/tunnelcalibration/dep"

type DeleteEventNote struct{ Auth *dep.Service }

func (c *DeleteEventNote) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
