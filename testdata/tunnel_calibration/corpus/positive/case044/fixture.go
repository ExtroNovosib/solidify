package fixture

import "example.com/tunnelcalibration/dep"

type CreateInvite struct{ Auth *dep.Service }

func (c *CreateInvite) Handle() error {
	if c.Auth == nil {
		return nil
	}
	return c.Auth.Handle()
}
