package main

import (
	"bytes"
	"cmp"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
)

var (
	//go:embed index.html.template
	indexHTMLTemplate string

	//go:embed data.json
	dataJSON []byte
)

func main() {
	mux := http.NewServeMux()
	server := new(server)
	server.templates = template.Must(template.New("index.html").Parse(indexHTMLTemplate))
	if err := json.Unmarshal(dataJSON, &server.Rows); err != nil {
		log.Fatal(err)
	}
	mux.HandleFunc("GET /", server.index)
	mux.HandleFunc("GET /edit/{index}", server.getEdit)
	mux.HandleFunc("POST /edit/{index}", server.postEdit)
	log.Fatal(http.ListenAndServe(":"+cmp.Or(os.Getenv("PORT"), ":8080"), mux))
}

type server struct {
	sync.Mutex
	templates *template.Template
	Rows      []Row
}

func (server *server) updateRow(index int, name, email string) ([]Row, error) {
	server.Lock()
	defer server.Unlock()
	if index < 0 || index > len(server.Rows) {
		return nil, fmt.Errorf("index out of range")
	}
	server.Rows[index].Name = name
	server.Rows[index].Email = email
	return slices.Clone(server.Rows), nil
}

type Row struct {
	Name  string
	Email string
}

func (server *server) render(res http.ResponseWriter, _ *http.Request, templateName string, data any) {
	var buf bytes.Buffer
	if err := server.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
	_, _ = res.Write(buf.Bytes())
}

func (server *server) index(res http.ResponseWriter, req *http.Request) {
	server.render(res, req, "index.html", struct {
		Rows []Row
	}{
		Rows: slices.Clone(server.Rows),
	})
}

func (server *server) getEdit(res http.ResponseWriter, req *http.Request) {
	index, err := strconv.Atoi(req.PathValue("index"))
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	server.Mutex.Lock()
	rows := slices.Clone(server.Rows)
	server.Mutex.Unlock()
	server.render(res, req, "edit-rows", struct {
		Rows        []Row
		EditedIndex int
	}{
		Rows:        rows,
		EditedIndex: index,
	})
}

func (server *server) postEdit(res http.ResponseWriter, req *http.Request) {
	index, err := strconv.Atoi(req.PathValue("index"))
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	if err := req.ParseForm(); err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	var (
		name  = req.Form.Get("name")
		email = req.Form.Get("email")
	)
	rows, err := server.updateRow(index, name, email)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}
	server.render(res, req, "display-rows", struct {
		Rows []Row
	}{
		Rows: rows,
	})
}
