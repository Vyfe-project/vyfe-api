
// Sample sessionshelf is a fully-featured app demonstrating several Google Cloud APIs, including Datastore, Cloud SQL, Cloud Storage.
// See https://cloud.google.com/go/getting-started/tutorial-app
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"

	"golang.org/x/net/context"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	uuid "github.com/satori/go.uuid"

	"google.golang.org/appengine"

	"github.com/GoogleCloudPlatform/golang-samples/getting-started/vyfe-api"
)

var (
	// See template.go
	listTmpl   = parseTemplate("list.html")
	editTmpl   = parseTemplate("edit.html")
	detailTmpl = parseTemplate("detail.html")
)

func main() {
	registerHandlers()
	appengine.Main()
}

func registerHandlers() {
	// Use gorilla/mux for rich routing.
	// See http://www.gorillatoolkit.org/pkg/mux
	r := mux.NewRouter()

	r.Handle("/", http.RedirectHandler("/sessions", http.StatusFound))

	r.Methods("GET").Path("/sessions").
		Handler(appHandler(listHandler))
	r.Methods("GET").Path("/sessions/{id:[0-9]+}").
		Handler(appHandler(detailHandler))
	r.Methods("GET").Path("/sessions/add").
		Handler(appHandler(addFormHandler))
	r.Methods("GET").Path("/sessions/{id:[0-9]+}/edit").
		Handler(appHandler(editFormHandler))

	r.Methods("POST").Path("/sessions").
		Handler(appHandler(createHandler))
	r.Methods("POST", "PUT").Path("/sessions/{id:[0-9]+}").
		Handler(appHandler(updateHandler))
	r.Methods("POST").Path("/sessions/{id:[0-9]+}:delete").
		Handler(appHandler(deleteHandler)).Name("delete")

	// The following handlers are defined in auth.go and used in the
	// "Authenticating Users" part of the Getting Started guide.
	r.Methods("GET").Path("/login").
		Handler(appHandler(loginHandler))
	r.Methods("POST").Path("/logout").
		Handler(appHandler(logoutHandler))
	r.Methods("GET").Path("/oauth2callback").
		Handler(appHandler(oauthCallbackHandler))

	// Respond to App Engine and Compute Engine health checks.
	// Indicate the server is healthy.
	r.Methods("GET").Path("/_ah/health").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})

	// [START request_logging]
	// Delegate all of the HTTP routing and serving to the gorilla/mux router.
	// Log all requests using the standard Apache format.
	http.Handle("/", handlers.CombinedLoggingHandler(os.Stderr, r))
	// [END request_logging]
}

// listHandler displays a list with summaries of sessions in the database.
func listHandler(w http.ResponseWriter, r *http.Request) *appError {
	sessions, err := vyfe_api.DB.ListSessions()
	if err != nil {
		return appErrorf(err, "could not list sessions: %v", err)
	}

	return listTmpl.Execute(w, r, sessions)
}

// listMineHandler displays a list of sessions created by the currently
// authenticated user.
func listMineHandler(w http.ResponseWriter, r *http.Request) *appError {
	user := profileFromSession(r)
	if user == nil {
		http.Redirect(w, r, "/login?redirect=/sessions/mine", http.StatusFound)
		return nil
	}

	sessions, err := vyfe_api.DB.ListSessionsCreatedBy(user.ID)
	if err != nil {
		return appErrorf(err, "could not list sessions: %v", err)
	}

	return listTmpl.Execute(w, r, sessions)
}

// sessionFromRequest retrieves a session from the database given a session ID in the
// URL's path.
func sessionFromRequest(r *http.Request) (*vyfe_api.Session, error) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("bad session id: %v", err)
	}
	session, err := vyfe_api.DB.GetSession(id)
	if err != nil {
		return nil, fmt.Errorf("could not find session: %v", err)
	}
	return session, nil
}

// detailHandler displays the details of a given session.
func detailHandler(w http.ResponseWriter, r *http.Request) *appError {
	session, err := sessionFromRequest(r)
	if err != nil {
		return appErrorf(err, "%v", err)
	}

	return detailTmpl.Execute(w, r, session)
}

// addFormHandler displays a form that captures details of a new session to add to
// the database.
func addFormHandler(w http.ResponseWriter, r *http.Request) *appError {
	return editTmpl.Execute(w, r, nil)
}

// editFormHandler displays a form that allows the user to edit the details of
// a given session.
func editFormHandler(w http.ResponseWriter, r *http.Request) *appError {
	session, err := sessionFromRequest(r)
	if err != nil {
		return appErrorf(err, "%v", err)
	}

	return editTmpl.Execute(w, r, session)
}

// sessionFromForm populates the fields of a Session from form values
// (see templates/edit.html).
func sessionFromForm(r *http.Request) (*vyfe_api.Session, error) {
	videoURL, err := uploadFileFromForm(r)
	if err != nil {
		return nil, fmt.Errorf("could not upload file: %v", err)
	}
	if videoURL == "" {
		videoURL = r.FormValue("videoURL")
	}

	session := &vyfe_api.Session{
		Title:         r.FormValue("title"),
		Author:        r.FormValue("author"),
		PublishedDate: r.FormValue("publishedDate"),
		VideoURL:      videoURL,
		Description:   r.FormValue("description"),
		CreatedBy:     r.FormValue("createdBy"),
		CreatedByID:   r.FormValue("createdByID"),
	}

	// If the form didn't carry the user information for the creator, populate it
	// from the currently logged in user (or mark as anonymous).
	if session.CreatedByID == "" {
		user := profileFromSession(r)
		if user != nil {
			// Logged in.
			session.CreatedBy = user.DisplayName
			session.CreatedByID = user.ID
		} else {
			// Not logged in.
			session.SetCreatorAnonymous()
		}
	}

	return session, nil
}

// uploadFileFromForm uploads a file if it's present in the "image" form field.
func uploadFileFromForm(r *http.Request) (url string, err error) {
	f, fh, err := r.FormFile("image")
	if err == http.ErrMissingFile {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	if vyfe_api.StorageBucket == nil {
		return "", errors.New("storage bucket is missing - check config.go")
	}

	// random filename, retaining existing extension.
	name := uuid.Must(uuid.NewV4()).String() + path.Ext(fh.Filename)

	ctx := context.Background()
	w := vyfe_api.StorageBucket.Object(name).NewWriter(ctx)
	w.ACL = []storage.ACLRule{{Entity: storage.AllUsers, Role: storage.RoleReader}}
	w.ContentType = fh.Header.Get("Content-Type")

	// Entries are immutable, be aggressive about caching (1 day).
	w.CacheControl = "public, max-age=86400"

	if _, err := io.Copy(w, f); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	const publicURL = "https://storage.googleapis.com/%s/%s"
	return fmt.Sprintf(publicURL, vyfe_api.StorageBucketName, name), nil
}

// createHandler adds a session to the database.
func createHandler(w http.ResponseWriter, r *http.Request) *appError {
	session, err := sessionFromForm(r)
	if err != nil {
		return appErrorf(err, "could not parse session from form: %v", err)
	}
	id, err := vyfe_api.DB.AddSession(session)
	if err != nil {
		return appErrorf(err, "could not save session: %v", err)
	}
	go publishUpdate(id)
	http.Redirect(w, r, fmt.Sprintf("/sessions/%d", id), http.StatusFound)
	return nil
}

// updateHandler updates the details of a given session.
func updateHandler(w http.ResponseWriter, r *http.Request) *appError {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		return appErrorf(err, "bad session id: %v", err)
	}

	session, err := sessionFromForm(r)
	if err != nil {
		return appErrorf(err, "could not parse session from form: %v", err)
	}
	session.ID = id

	err = vyfe_api.DB.UpdateSession(session)
	if err != nil {
		return appErrorf(err, "could not save session: %v", err)
	}
	go publishUpdate(session.ID)
	http.Redirect(w, r, fmt.Sprintf("/sessions/%d", session.ID), http.StatusFound)
	return nil
}

// deleteHandler deletes a given session.
func deleteHandler(w http.ResponseWriter, r *http.Request) *appError {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		return appErrorf(err, "bad session id: %v", err)
	}
	err = vyfe_api.DB.DeleteSession(id)
	if err != nil {
		return appErrorf(err, "could not delete session: %v", err)
	}
	http.Redirect(w, r, "/sessions", http.StatusFound)
	return nil
}

// publishUpdate notifies Pub/Sub subscribers that the session identified with
// the given ID has been added/modified.
func publishUpdate(sessionID int64) {
	if vyfe_api.PubsubClient == nil {
		return
	}

	ctx := context.Background()

	b, err := json.Marshal(sessionID)
	if err != nil {
		return
	}
	topic := vyfe_api.PubsubClient.Topic(vyfe_api.PubsubTopicID)
	_, err = topic.Publish(ctx, &pubsub.Message{Data: b}).Get(ctx)
	log.Printf("Published update to Pub/Sub for Session ID %d: %v", sessionID, err)
}

// http://blog.golang.org/error-handling-and-go
type appHandler func(http.ResponseWriter, *http.Request) *appError

type appError struct {
	Error   error
	Message string
	Code    int
}

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if e := fn(w, r); e != nil { // e is *appError, not os.Error.
		log.Printf("Handler error: status code: %d, message: %s, underlying err: %#v",
			e.Code, e.Message, e.Error)

		http.Error(w, e.Message, e.Code)
	}
}

func appErrorf(err error, format string, v ...interface{}) *appError {
	return &appError{
		Error:   err,
		Message: fmt.Sprintf(format, v...),
		Code:    500,
	}
}
