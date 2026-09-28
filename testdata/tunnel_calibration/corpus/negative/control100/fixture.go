package fixture

import "example.com/tunnelcalibration/dep"

type Result struct{ Value *dep.Aggregate }
type Store interface{ Get() Result }

func Project(a *dep.Aggregate) Result { return Result{a} }
