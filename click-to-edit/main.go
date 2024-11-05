package main

import (
	"bytes"
	"cmp"
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/crhntr/httplog"
	_ "github.com/mattn/go-sqlite3"

	"github.com/crhntr/go-htmx-examples/click-to-edit/internal/database"
)

//go:generate sqlc generate
//go:generate counterfeiter -generate

//counterfeiter:generate -o internal/fakes --fake-name Querier internal/database Querier

func main() {
	db := must(sql.Open("sqlite3", ":memory:"))
	_ = must(db.ExecContext(context.Background(), schemaSQL))
	server := newServer(database.New(db))
	h := httplog.Wrap(server.routes())
	log.Println("starting server")
	log.Fatal(http.ListenAndServe(":"+cmp.Or(os.Getenv("PORT"), "8080"), h))
}

func must[T any](value T, err error) T {
	if err != nil {
		log.Panicln(err)
	}
	return value
}

var (
	//go:embed contact.html.template
	contactPages string

	//go:embed schema.sql
	schemaSQL string
)

type Server struct {
	templates *template.Template
	db        database.Querier
}

func newServer(db database.Querier) *Server {
	server := &Server{
		db: db,
	}
	templates := template.Must(template.New("").Funcs(server.templateFunctions()).Parse(contactPages))
	server.templates = templates
	return server
}

func (server *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", server.index)
	mux.HandleFunc("GET /contact/{id}", server.handleContactID(server.view))
	mux.HandleFunc("GET /contact/{id}/edit", server.handleContactID(server.edit))
	mux.HandleFunc("POST /contact/{id}", server.handleContactID(server.submit))
	return mux
}

func (server *Server) templateFunctions() template.FuncMap {
	return template.FuncMap{
		"execute": server.execute,
	}
}

func (server *Server) execute(name string, data any) (template.HTML, error) {
	var buf bytes.Buffer
	err := server.templates.ExecuteTemplate(&buf, name, data)
	return template.HTML(buf.String()), err
}

func (server *Server) index(res http.ResponseWriter, req *http.Request) {
	contacts, err := server.db.ListContacts(req.Context())
	if err != nil {
		server.writeError(res, err)
		return
	}
	server.writePage(res, req, "list-contacts", http.StatusOK, contacts)
}

func (server *Server) handleContactID(next func(http.ResponseWriter, *http.Request, int64)) http.HandlerFunc {
	return func(res http.ResponseWriter, req *http.Request) {
		id, err := strconv.Atoi(req.PathValue("id"))
		if err != nil {
			server.writeError(res, StatusError{
				Status: http.StatusBadRequest,
				Err:    fmt.Errorf("failed to parse contact id: %w", err),
			})
			return
		}
		next(res, req, int64(id))
	}
}

func (server *Server) view(res http.ResponseWriter, req *http.Request, id int64) {
	contact, err := server.db.ContactWithID(req.Context(), id)
	if err != nil {
		server.writeError(res, err)
		return
	}
	server.writePage(res, req, "view-contact", http.StatusOK, contact)
}

func (server *Server) edit(res http.ResponseWriter, req *http.Request, id int64) {
	contact, err := server.db.ContactWithID(req.Context(), id)
	if err != nil {
		server.writeError(res, err)
		return
	}
	server.writePage(res, req, "edit-contact", http.StatusOK, contact)
}

func (server *Server) submit(res http.ResponseWriter, req *http.Request, id int64) {
	if err := req.ParseForm(); err != nil {
		server.writeError(res, StatusError{
			Status: http.StatusBadRequest,
			Err:    err,
		})
		return
	}
	ctx := req.Context()
	if err := server.db.UpdateContact(ctx, database.UpdateContactParams{
		ID:        id,
		FirstName: req.Form.Get("first-name"),
		LastName:  req.Form.Get("last-name"),
		Email:     req.Form.Get("email"),
	}); err != nil {
		server.writeError(res, err)
		return
	}
	contact, err := server.db.ContactWithID(ctx, id)
	if err != nil {
		server.writeError(res, err)
		return
	}
	server.writePage(res, req, "view-contact", http.StatusOK, contact)
}

func (server *Server) writePage(res http.ResponseWriter, req *http.Request, name string, status int, data any) {
	type PageData struct {
		PageName string
		Data     any
	}

	var (
		buf bytes.Buffer
		err error
	)
	if req.Header.Get("hx-target") == "view" {
		err = server.templates.ExecuteTemplate(&buf, name, data)
	} else {
		err = server.templates.ExecuteTemplate(&buf, "page", PageData{
			PageName: name,
			Data:     data,
		})
	}
	if err != nil {
		log.Println(err)
		http.Error(res, "failed to write page", http.StatusInternalServerError)
		return
	}

	res.Header().Set("content-type", "test/html; charset=utf-8")
	res.WriteHeader(status)
	_, _ = res.Write(buf.Bytes())
}

type StatusError struct {
	Status int
	Err    error
}

func (err StatusError) Error() string { return err.Err.Error() }

func (server *Server) writeError(res http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var se StatusError
	if errors.As(err, &se) {
		status = se.Status
	} else if errors.Is(err, sql.ErrNoRows) {
		status = http.StatusNotFound
	}
	http.Error(res, err.Error(), status)
}
