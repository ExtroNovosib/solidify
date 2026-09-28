package dep

type Service struct{}

func (*Service) Handle() error         { return nil }
func (*Service) Execute() error        { return nil }
func (*Service) EvaluateSignal() error { return nil }
func (*Service) EvaluateDue() error    { return nil }

type Aggregate struct{ Value string }

func (a *Aggregate) Name() string { return a.Value }
func (a *Aggregate) ID() string   { return a.Value }

func NewService() *Service { return &Service{} }

type Foreign0 struct{ Value0 string }
type Foreign1 struct{ Value1 string }
type Foreign2 struct{ Value2 string }
type Foreign3 struct{ Value3 string }
type Foreign4 struct{ Value4 string }
type Foreign5 struct{ Value5 string }
