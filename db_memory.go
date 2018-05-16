
package vyfe_api

import (
	"errors"
	"fmt"
	"sort"
	"sync"
)

// Ensure memoryDB conforms to the SessionDatabase interface.
var _ SessionDatabase = &memoryDB{}

// memoryDB is a simple in-memory persistence layer for sessions.
type memoryDB struct {
	mu     sync.Mutex
	nextID int64           // next ID to assign to a session.
	sessions  map[int64]*Session // maps from Session ID to Session.
}

func newMemoryDB() *memoryDB {
	return &memoryDB{
		sessions:  make(map[int64]*Session),
		nextID: 1,
	}
}

// Close closes the database.
func (db *memoryDB) Close() {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.sessions = nil
}

// GetSession retrieves a session by its ID.
func (db *memoryDB) GetSession(id int64) (*Session, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	session, ok := db.sessions[id]
	if !ok {
		return nil, fmt.Errorf("memorydb: session not found with ID %d", id)
	}
	return session, nil
}

// AddSession saves a given session, assigning it a new ID.
func (db *memoryDB) AddSession(b *Session) (id int64, err error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	b.ID = db.nextID
	db.sessions[b.ID] = b

	db.nextID++

	return b.ID, nil
}

// DeleteSession removes a given session by its ID.
func (db *memoryDB) DeleteSession(id int64) error {
	if id == 0 {
		return errors.New("memorydb: session with unassigned ID passed into deleteSession")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	if _, ok := db.sessions[id]; !ok {
		return fmt.Errorf("memorydb: could not delete session with ID %d, does not exist", id)
	}
	delete(db.sessions, id)
	return nil
}

// UpdateSession updates the entry for a given session.
func (db *memoryDB) UpdateSession(b *Session) error {
	if b.ID == 0 {
		return errors.New("memorydb: session with unassigned ID passed into updateSession")
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	db.sessions[b.ID] = b
	return nil
}

// sessionsByTitle implements sort.Interface, ordering sessions by Title.
// https://golang.org/pkg/sort/#example__sortWrapper
type sessionsByTitle []*Session

func (s sessionsByTitle) Less(i, j int) bool { return s[i].Title < s[j].Title }
func (s sessionsByTitle) Len() int           { return len(s) }
func (s sessionsByTitle) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// ListSessions returns a list of sessions, ordered by title.
func (db *memoryDB) ListSessions() ([]*Session, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	var sessions []*Session
	for _, b := range db.sessions {
		sessions = append(sessions, b)
	}

	sort.Sort(sessionsByTitle(sessions))
	return sessions, nil
}

// ListSessionsCreatedBy returns a list of sessions, ordered by title, filtered by
// the user who created the session entry.
func (db *memoryDB) ListSessionsCreatedBy(userID string) ([]*Session, error) {
	if userID == "" {
		return db.ListSessions()
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	var sessions []*Session
	for _, b := range db.sessions {
		if b.CreatedByID == userID {
			sessions = append(sessions, b)
		}
	}

	sort.Sort(sessionsByTitle(sessions))
	return sessions, nil
}
