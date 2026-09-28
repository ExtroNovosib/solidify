package fixture

import "example.com/tunnelcalibration/dep"

type CreateShareLink struct{ Auth *dep.Service }

func (c *CreateShareLink) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
