package embedding

type Reader interface {
	Open()
	Read()
	Seek()
	Close()
}

type Writer interface {
	Create()
	Write()
	Sync()
	Remove()
	Rename()
}

// Aggregate intentionally embeds two focused roles. Its aggregate surface is
// still wider than the configured maximum and should remain visible as a
// deliberate boundary decision.
type Aggregate interface {
	Reader
	Writer
}
