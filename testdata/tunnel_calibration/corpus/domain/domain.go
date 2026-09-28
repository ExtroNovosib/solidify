package domain

type Aggregate struct{ value string }

func New() *Aggregate                 { return &Aggregate{} }
func (a *Aggregate) Publish(v string) { a.value = v }
func (a *Aggregate) Name() string     { return a.value }
func (a *Aggregate) ID() string       { return a.value }

type Format string

const (
	JSON Format = "json"
	Raw  Format = "raw"
	Curl Format = "curl"
)
