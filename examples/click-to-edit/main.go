package main

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/crhntr/httplog"
	"github.com/julienschmidt/httprouter"

	_ "github.com/mattn/go-sqlite3"

	"github.com/crhntr/go-mysql-htmx/examples/click-to-edit/internal/database"
)

func main() {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), initSQL); err != nil {
		log.Fatal(err)
	}

	server := newServer(db)
	mux := httprouter.New()
	mux.GET("/", server.index)
	mux.GET("/contact/:id", server.view)
	mux.GET("/contact/:id/edit", server.edit)
	mux.POST("/contact/:id", server.submit)
	log.Println("starting server")
	log.Fatal(http.ListenAndServe(":8080", httplog.Wrap(mux)))
}

var (
	//go:embed contact.html.template
	contactPages string

	//go:embed schema.sql
	initSQL string
)

type Server struct {
	templates *template.Template
	db        *database.Queries
}

func newServer(db database.DBTX) *Server {
	server := &Server{
		db: database.New(db),
	}
	templates := template.Must(template.New("").Funcs(server.templateFunctions()).Parse(contactPages))
	server.templates = templates
	return server
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

func (server *Server) index(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	contacts, err := server.db.ListContacts(req.Context())
	if err != nil {
		server.writeError(res, StatusError{
			Err:    err,
			Status: http.StatusInternalServerError,
		})
		return
	}
	server.writePage(res, req, "list-contacts", http.StatusOK, contacts)
}

func (server *Server) view(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	ctx := req.Context()
	id, err := server.contactID(params)
	if err != nil {
		server.writeError(res, err)
		return
	}
	contact, err := server.db.ContactWithID(ctx, id)
	if err != nil {
		server.writeError(res, err)
	}
	server.writePage(res, req, "view-contact", http.StatusOK, contact)
}

func (server *Server) edit(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	ctx := req.Context()
	id, err := server.contactID(params)
	if err != nil {
		server.writeError(res, err)
		return
	}
	contact, err := server.db.ContactWithID(ctx, id)
	if err != nil {
		server.writeError(res, err)
	}

	server.writePage(res, req, "edit-contact", http.StatusOK, contact)
}

func (server *Server) submit(res http.ResponseWriter, req *http.Request, params httprouter.Params) {
	if err := req.ParseForm(); err != nil {
		server.writeError(res, err)
		return
	}
	ctx := req.Context()
	id, err := server.contactID(params)
	if err != nil {
		server.writeError(res, err)
		return
	}

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
		http.Error(res, "failed to writePage page", http.StatusInternalServerError)
		return
	}

	res.Header().Set("content-type", "text/html")
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
	}
	http.Error(res, err.Error(), status)
}

func (server *Server) contactID(params httprouter.Params) (int64, error) {
	id, err := strconv.Atoi(params.ByName("id"))
	if err != nil {
		return 0, StatusError{Status: http.StatusBadRequest, Err: fmt.Errorf("failed to parse contact id: %w", err)}
	}
	return int64(id), nil
}
