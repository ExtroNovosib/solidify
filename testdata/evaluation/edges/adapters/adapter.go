package adapters

// Client is intentionally small. The adapter owns translation at an external
// boundary and should not produce a SOLID finding by itself.
type Client interface {
	Get(string) (string, error)
}

type HTTPAdapter struct {
	client Client
}

func (adapter HTTPAdapter) Fetch(path string) (string, error) {
	return adapter.client.Get(path)
}
