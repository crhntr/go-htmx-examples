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
	"github.com/julienschmidt/httprouter"

	_ "github.com/mattn/go-sqlite3"

	"github.com/crhntr/go-mysql-htmx/examples/bulk-update/internal/database"
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
	mux := httprouter.New()
	mux.GET("/", server.index)
	mux.PUT("/activate", server.activate)
	mux.PUT("/deactivate", server.deactivate)
	return mux
}

func (server *Server) index(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	ctx := req.Context()
	colors, err := server.db.ListColors(ctx)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	server.writePage(res, req, "index.html.template", http.StatusOK, colors)
}

func (server *Server) activate(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	ctx := req.Context()
	_ = req.ParseForm()
	for _, idStr := range req.Form["ids"] {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		_ = server.db.SetStatus(ctx, database.SetStatusParams{
			ID:     int64(id),
			Active: true,
		})
	}
	colors, err := server.db.ListColors(ctx)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	server.writePage(res, req, "rows", http.StatusOK, colors)
}

func (server *Server) deactivate(res http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	ctx := req.Context()
	_ = req.ParseForm()
	for _, idStr := range req.Form["ids"] {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		_ = server.db.SetStatus(ctx, database.SetStatusParams{
			ID:     int64(id),
			Active: false,
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
