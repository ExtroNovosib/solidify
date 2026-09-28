package fixture

type Store interface {
	MaxNoteLength()
	MaxNotesPerEvent()
	EventGrantMaxLifetime()
	MaxEnvironments()
	MaxMembers()
	MaxShareLinks()
	ShareLinkMaxLifetime()
}
type Consumer struct{ store Store }

func (c *Consumer) Handle() { c.store.MaxNoteLength(); c.store.MaxNotesPerEvent() }
