

package vyfe_api

// Session holds metadata about a book.
type Session struct {
	ID            int64
	Title         string
	Author        string
	PublishedDate string
	VideoURL      string
	Description   string
	CreatedBy     string
	CreatedByID   string
}

// CreatedByDisplayName returns a string appropriate for displaying the name of
// the user who created this book object.
func (b *Session) CreatedByDisplayName() string {
	if b.CreatedByID == "anonymous" {
		return "Anonymous"
	}
	return b.CreatedBy
}

// SetCreatorAnonymous sets the CreatedByID field to the "anonymous" ID.
func (b *Session) SetCreatorAnonymous() {
	b.CreatedBy = ""
	b.CreatedByID = "anonymous"
}

// SessionDatabase provides thread-safe access to a database of sessions.
type SessionDatabase interface {
	// ListBooks returns a list of books, ordered by title.
	ListSessions() ([]*Session, error)

	// ListSessionsCreatedBy returns a list of sessions, ordered by title, filtered by
	// the user who created the session entry.
	ListSessionsCreatedBy(userID string) ([]*Session, error)

	// GetSession retrieves a book by its ID.
	GetSession(id int64) (*Session, error)

	// AddSession saves a given book, assigning it a new ID.
	AddSession(b *Session) (id int64, err error)

	// DeleteBook removes a given book by its ID.
	DeleteSession(id int64) error

	// UpdateBook updates the entry for a given book.
	UpdateSession(b *Session) error

	// Close closes the database, freeing up any available resources.
	// TODO(cbro): Close() should return an error.
	Close()
}
