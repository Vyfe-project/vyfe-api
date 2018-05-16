
package vyfe_api

import (
	"fmt"

	"cloud.google.com/go/datastore"

	"golang.org/x/net/context"
)

// datastoreDB persists sessions to Cloud Datastore.
// https://cloud.google.com/datastore/docs/concepts/overview
type datastoreDB struct {
	client *datastore.Client
}

// Ensure datastoreDB conforms to the SessionDatabase interface.
var _ SessionDatabase = &datastoreDB{}

// newDatastoreDB creates a new SessionDatabase backed by Cloud Datastore.
// See the datastore and google packages for details on creating a suitable Client:
// https://godoc.org/cloud.google.com/go/datastore
func newDatastoreDB(client *datastore.Client) (SessionDatabase, error) {
	ctx := context.Background()
	// Verify that we can communicate and authenticate with the datastore service.
	t, err := client.NewTransaction(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastoredb: could not connect: %v", err)
	}
	if err := t.Rollback(); err != nil {
		return nil, fmt.Errorf("datastoredb: could not connect: %v", err)
	}
	return &datastoreDB{
		client: client,
	}, nil
}

// Close closes the database.
func (db *datastoreDB) Close() {
	// No op.
}

func (db *datastoreDB) datastoreKey(id int64) *datastore.Key {
	return datastore.IDKey("Session", id, nil)
}

// GetSession retrieves a session by its ID.
func (db *datastoreDB) GetSession(id int64) (*Session, error) {
	ctx := context.Background()
	k := db.datastoreKey(id)
	session := &Session{}
	if err := db.client.Get(ctx, k, session); err != nil {
		return nil, fmt.Errorf("datastoredb: could not get Session: %v", err)
	}
	session.ID = id
	return session, nil
}

// AddSession saves a given session, assigning it a new ID.
func (db *datastoreDB) AddSession(b *Session) (id int64, err error) {
	ctx := context.Background()
	k := datastore.IncompleteKey("Session", nil)
	k, err = db.client.Put(ctx, k, b)
	if err != nil {
		return 0, fmt.Errorf("datastoredb: could not put Session: %v", err)
	}
	return k.ID, nil
}

// DeleteSession removes a given session by its ID.
func (db *datastoreDB) DeleteSession(id int64) error {
	ctx := context.Background()
	k := db.datastoreKey(id)
	if err := db.client.Delete(ctx, k); err != nil {
		return fmt.Errorf("datastoredb: could not delete Session: %v", err)
	}
	return nil
}

// UpdateSession updates the entry for a given session.
func (db *datastoreDB) UpdateSession(b *Session) error {
	ctx := context.Background()
	k := db.datastoreKey(b.ID)
	if _, err := db.client.Put(ctx, k, b); err != nil {
		return fmt.Errorf("datastoredb: could not update Session: %v", err)
	}
	return nil
}

// ListSessions returns a list of sessions, ordered by title.
func (db *datastoreDB) ListSessions() ([]*Session, error) {
	ctx := context.Background()
	sessions := make([]*Session, 0)
	q := datastore.NewQuery("Session").
		Order("Title")

	keys, err := db.client.GetAll(ctx, q, &sessions)

	if err != nil {
		return nil, fmt.Errorf("datastoredb: could not list sessions: %v", err)
	}

	for i, k := range keys {
		sessions[i].ID = k.ID
	}

	return sessions, nil
}

// ListSessionsCreatedBy returns a list of sessions, ordered by title, filtered by
// the user who created the session entry.
func (db *datastoreDB) ListSessionsCreatedBy(userID string) ([]*Session, error) {
	ctx := context.Background()
	if userID == "" {
		return db.ListSessions()
	}

	sessions := make([]*Session, 0)
	q := datastore.NewQuery("Session").
		Filter("CreatedByID =", userID).
		Order("Title")

	keys, err := db.client.GetAll(ctx, q, &sessions)

	if err != nil {
		return nil, fmt.Errorf("datastoredb: could not list sessions: %v", err)
	}

	for i, k := range keys {
		sessions[i].ID = k.ID
	}

	return sessions, nil
}
