package main

import (
	"bytes"
	"context"
	"database/sql"
	"embed"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/crhntr/httplog"
	_ "github.com/mattn/go-sqlite3"

	"github.com/crhntr/go-htmx-examples/bulk-update/internal/database"
)

//go:generate sqlc generate
//go:generate counterfeiter -generate

//go:embed *.html.template
var templateFS embed.FS

//go:embed schema.sql
var schemaSQL string

func main() {
	db := must(sql.Open("sqlite3", ":memory:"))
	_ = must(db.ExecContext(context.Background(), schemaSQL))
	server := Server{
		templates: template.Must(template.ParseFS(templateFS, "*")),
		db:        database.New(db),
	}
	mux := server.routes()
	h := httplog.Wrap(mux)
	log.Println("starting server")
	log.Fatal(http.ListenAndServe(":8080", h))
}

func must[T any](value T, err error) T {
	if err != nil {
		log.Panicln(err)
	}
	return value
}

type Server struct {
	templates *template.Template
	db        database.Querier
}

func (server *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", server.index)
	mux.HandleFunc("PUT /activate", server.activate)
	mux.HandleFunc("PUT /deactivate", server.deactivate)
	return mux
}

func (server *Server) index(res http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	colors, err := server.db.ListColors(ctx)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	server.writePage(res, req, "index.html.template", http.StatusOK, colors)
}

func (server *Server) activate(res http.ResponseWriter, req *http.Request) {
	server.setActiveStatus(res, req, true)
}

func (server *Server) deactivate(res http.ResponseWriter, req *http.Request) {
	server.setActiveStatus(res, req, false)
}

func (server *Server) setActiveStatus(res http.ResponseWriter, req *http.Request, active bool) {
	ctx := req.Context()
	_ = req.ParseForm()
	for _, idStr := range req.Form["ids"] {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		_ = server.db.SetStatus(ctx, database.SetStatusParams{
			ID:     int64(id),
			Active: active,
		})
	}
	colors, err := server.db.ListColors(ctx)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	server.writePage(res, req, "rows", http.StatusOK, colors)
}

func (server *Server) writePage(res http.ResponseWriter, _ *http.Request, name string, status int, data any) {
	var (
		buf bytes.Buffer
	)
	err := server.templates.ExecuteTemplate(&buf, name, data)
	if err != nil {
		log.Println(err)
		http.Error(res, "failed to write page", http.StatusInternalServerError)
		return
	}
	res.Header().Set("content-type", "text/html")
	res.WriteHeader(status)
	_, _ = res.Write(buf.Bytes())
}
