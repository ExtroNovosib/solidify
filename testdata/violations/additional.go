package violations

import "tempmod/database"

// ---- SRP violation: a function that has grown beyond one reviewable unit ----

func ReconcileLedger() {
	processed := 0
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	processed++
	_ = processed
}

// ---- OCP violation: an if/else-if type-assertion chain ----

type EventCreated struct{}
type EventUpdated struct{}
type EventDeleted struct{}
type EventArchived struct{}
type EventRestored struct{}

func HandleEvent(event any) {
	if _, ok := event.(EventCreated); ok {
		return
	} else if _, ok := event.(EventUpdated); ok {
		return
	} else if _, ok := event.(EventDeleted); ok {
		return
	} else if _, ok := event.(EventArchived); ok {
		return
	} else if _, ok := event.(EventRestored); ok {
		return
	}
}

// ---- ISP violation: embedded interfaces combine into a fat interface ----

type ReaderOperations interface {
	Read()
	Peek()
}

type WriterOperations interface {
	Write()
	Flush()
}

type FullDuplexOperations interface {
	ReaderOperations
	WriterOperations
	Close()
	Reset()
	Sync()
	Drain()
	Seek()
}

// ---- DIP violation: a constructor accepts a concrete dependency ----

type ArchiveService struct{}

func NewArchiveService(client *database.PostgreSQLClient) *ArchiveService {
	_ = client
	return &ArchiveService{}
}
