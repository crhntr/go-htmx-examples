package main

import (
	"bytes"
	"cmp"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
)

//go:embed index.html.template
var indexHTMLTemplate string

const rowsPerPage = 100

func main() {
	templates := template.Must(template.New("").Funcs(template.FuncMap{
		"lastIndex": func(length, index int) bool {
			return index == (length - 1)
		},
	}).Parse(indexHTMLTemplate))
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		const initialID = 1
		render(res, templates, Page{
			Rows:    setIDs(initialID, make([]Contact, rowsPerPage)),
			NextURL: contactsURL(initialID),
		})
	})
	mux.HandleFunc("/contacts", func(res http.ResponseWriter, req *http.Request) {
		var page int
		if p, err := strconv.Atoi(req.URL.Query().Get("page")); err == nil {
			page = p
		}
		render(res, templates.Lookup("rows"), Page{
			Rows:    setIDs(page, make([]Contact, rowsPerPage)),
			NextURL: contactsURL(page + 1),
		})
	})
	log.Fatal(http.ListenAndServe(":"+cmp.Or(os.Getenv("PORT"), ":8080"), mux))
}

func contactsURL(page int) string {
	return fmt.Sprintf("/contacts?page=%d", page)
}

func render(res http.ResponseWriter, t *template.Template, data any) {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(http.StatusOK)
	_, _ = res.Write(buf.Bytes())
}

type Page struct {
	Rows    []Contact
	NextURL string
}

type Contact struct {
	ID int
}

func setIDs(n int, in []Contact) []Contact {
	for i := range in {
		in[i].ID = (n * 1000) + i
	}
	return in
}
