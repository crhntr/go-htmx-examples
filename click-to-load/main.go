package main

import (
	"bytes"
	"cmp"
	_ "embed"
	"html/template"
	"math/rand"
	"net/http"
	"os"
)

//go:embed index.html.template
var indexHTML string

func main() {
	ts := template.Must(template.New("").Parse(indexHTML))
	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		var numbers [10]int
		fillWithRandomNumbers(numbers[:])
		var buf bytes.Buffer
		_ = ts.Execute(&buf, numbers)
		res.WriteHeader(http.StatusOK)
		_, _ = res.Write(buf.Bytes())
	})
	http.HandleFunc("/more-rows", func(res http.ResponseWriter, req *http.Request) {
		var numbers [10]int
		fillWithRandomNumbers(numbers[:])
		var buf bytes.Buffer
		_ = ts.ExecuteTemplate(&buf, "rows", numbers)
		res.WriteHeader(http.StatusOK)
		_, _ = res.Write(buf.Bytes())
	})
	_ = http.ListenAndServe(":"+cmp.Or(os.Getenv("PORT"), ":8080"), nil)
}

func fillWithRandomNumbers(values []int) {
	for i := range values {
		values[i] = rand.Int()
	}
}
