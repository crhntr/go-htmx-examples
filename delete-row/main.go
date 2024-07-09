package main

import (
	"bytes"
	"cmp"
	_ "embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"slices"
	"strconv"
	"sync"
)

var (
	//go:embed data.json
	dataJSON []byte

	//go:embed index.html.template
	indexHTMLTemplate string
	indexHTML         = template.Must(template.New("index").Parse(indexHTMLTemplate))
)

func main() {
	var (
		rows []Row
		mut  sync.Mutex
	)
	if err := json.Unmarshal(dataJSON, &rows); err != nil {
		log.Fatal(err)
	}
	http.HandleFunc("/rows/", func(res http.ResponseWriter, req *http.Request) {
		n, err := strconv.Atoi(path.Base(req.URL.Path))
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		mut.Lock()
		defer mut.Unlock()
		if n < 0 || n >= len(rows) {
			http.Error(res, "index out of range", http.StatusBadRequest)
			return
		}
		rows = slices.Delete(rows, n, n+1)
		res.WriteHeader(http.StatusAccepted)
	})
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		buf := bytes.NewBuffer(make([]byte, len(indexHTMLTemplate)+2))
		mut.Lock()
		defer mut.Unlock()
		if err := indexHTML.Execute(buf, Page{
			Rows: rows,
		}); err != nil {
			http.Error(res, err.Error(), http.StatusInternalServerError)
			return
		}
		res.Header().Set("content-type", "text/html")
		res.WriteHeader(http.StatusOK)
		_, _ = res.Write(buf.Bytes())
	})
	log.Fatal(http.ListenAndServe(":"+cmp.Or(os.Getenv("PORT"), ":8080"), nil))
}

type Page struct {
	Rows []Row
}

type Row struct {
	Name,
	Emoji string
}
