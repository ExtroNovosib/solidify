package domainblob

// SessionRecord is a passive data carrier.
type SessionRecord struct {
	ID           string
	UserID       string
	Protocol     string
	Target       string
	PublicURL    string
	Status       string
	CreatedAt    string
	ExpiresAt    string
	LastActive   string
	CustomDomain string
	Region       string
	Tier         string
}

func (s *SessionRecord) GetID() string {
	return s.ID
}

func (s *SessionRecord) GetUserID() string {
	return s.UserID
}
