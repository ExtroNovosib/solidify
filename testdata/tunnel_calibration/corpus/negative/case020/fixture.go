package fixture

type Store interface {
	MaxNoteLength()
	MaxNotesPerEvent()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.MaxNoteLength(); c.store.MaxNotesPerEvent() }
