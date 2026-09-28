package fixture

import "example.com/tunnelcalibration/dep"

type BearerResolver struct{ AuthSrv *dep.Service }

func (c *BearerResolver) Handle() error {
	if c.AuthSrv == nil {
		return nil
	}
	return c.AuthSrv.Handle()
}
