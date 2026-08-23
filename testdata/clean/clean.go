package clean

type Notifier interface {
	Notify(msg string) error
}

type EmailNotifier struct{}

func (e *EmailNotifier) Notify(msg string) error { return nil }

type Order struct {
	notifier Notifier
}

func NewOrder(n Notifier) *Order {
	return &Order{notifier: n}
}

// A long, homogeneous parameter list can still express one cohesive
// operation. Parameter count alone is intentionally not an SRP finding.
func Sum(a, b, c, d, e, f int) int {
	return a + b + c + d + e + f
}
