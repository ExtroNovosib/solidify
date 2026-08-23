package sqlfake

type DB struct{}

func (db *DB) QueryUser(id string) string {
	return id
}

func (db *DB) InsertUser(name string) error {
	return nil
}

func (db *DB) DeleteUser(id string) error {
	return nil
}

func QueryRow(db DB, id string) string {
	return db.QueryUser(id)
}

func ExecInsert(db DB, name string) error {
	return db.InsertUser(name)
}

func ExecDelete(db DB, id string) error {
	return db.DeleteUser(id)
}
