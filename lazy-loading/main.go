package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		// language=html
		_, _ = io.WriteString(res, `<!DOCTYPE html>
<html lang="en">
<head>
  <title>Lazy Loading</title>
  <script src="https://unpkg.com/htmx.org@1.9.6"
          integrity="sha384-FhXw7b6AlE/jyjlZH5iHa/tTe9EpJ1Y55RjcgPbjeWMskSxZt1v9qkxLJWNJaGni"
          crossorigin="anonymous"></script>
</head>
<body>
	<div hx-get="/endpoint?sleep=2s" hx-trigger="load">
	  <div class="htmx-indicator">
		Loading...
	  </div>
	</div>
</body>
</html`)
	})
	mux.HandleFunc("/endpoint", func(res http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		sleep := time.Second * 3
		if in := q.Get("sleep"); in != "" {
			var err error
			sleep, err = time.ParseDuration(in)
			if err != nil {
				http.Error(res, err.Error(), http.StatusBadRequest)
				return
			}
			if sleep > time.Minute {
				sleep = time.Minute
			}
		}
		time.Sleep(sleep)
		res.WriteHeader(http.StatusOK)
		// language=html
		_, _ = io.WriteString(res, fmt.Sprintf(`<div>Waited %s.</div>`, sleep))
	})
	log.Fatal(http.ListenAndServe(":8080", mux))
}
