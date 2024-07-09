package main

import (
	"cmp"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/endpoint", endpointHandler)
	log.Fatal(http.ListenAndServe(":"+cmp.Or(os.Getenv("PORT"), ":8080"), mux))
}

func indexHandler(res http.ResponseWriter, _ *http.Request) {
	// language=html
	indexHTML := `<!DOCTYPE html>
<html lang="en">
<head>
  <title>Lazy Loading</title>
  <script src="https://unpkg.com/htmx.org@2.0.0"
          integrity="sha384-wS5l5IKJBvK6sPTKa2WZ1js3d947pvWXbPJ1OmWfEuxLgeHcEbjUUA5i9V5ZkpCw"
          crossorigin="anonymous"></script>
</head>
<body>
	<div hx-get="/endpoint?sleep=2s" hx-trigger="load">
	  <div class="htmx-indicator">
		Loading...
	  </div>
	</div>
</body>
</html>`
	res.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(res, indexHTML)
}

func endpointHandler(res http.ResponseWriter, req *http.Request) {
	sleep, ok := parseSleepQueryParameter(res, req)
	if !ok {
		return
	}
	time.Sleep(sleep)
	res.WriteHeader(http.StatusOK)
	// language=html
	_, _ = io.WriteString(res, fmt.Sprintf(`<div>Waited %s.</div>`, sleep))
}

func parseSleepQueryParameter(res http.ResponseWriter, req *http.Request) (time.Duration, bool) {
	const defaultValue = time.Second * 3
	q := req.URL.Query()
	in := q.Get("sleep")
	if in == "" {
		return defaultValue, true
	}
	result, err := time.ParseDuration(in)
	if err != nil {
		http.Error(res, err.Error(), http.StatusBadRequest)
		return 0, false
	}
	if result > time.Minute || result < 0 {
		http.Error(res, "sleep duration out of accepted range", http.StatusBadRequest)
		return 0, false
	}
	return result, true
}
