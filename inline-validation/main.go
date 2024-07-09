package main

import (
	"bytes"
	"cmp"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/crhntr/sse"
)

//go:embed index.html.template
var indexTemplateHTML string

func main() {
	templates := template.Must(template.New("index").Parse(indexTemplateHTML))

	type State struct {
		N        int
		Duration time.Duration
		Step     int
	}
	updateDuration := make(chan time.Duration)
	updateStep := make(chan int)
	countUpdates := make(chan int)
	stateRequest := make(chan chan State)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		var (
			n = 0
			d = time.Second
			s = 1
		)
		ticker := time.NewTicker(d)
		for {
			select {
			case <-ticker.C:
				n += s
				countUpdates <- n
			case updated := <-updateDuration:
				ticker.Reset(updated)
				d = updated
			case updated := <-updateStep:
				s = updated
			case req := <-stateRequest:
				req <- State{
					N:        n,
					Duration: d,
					Step:     s,
				}
				close(req)
			}
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/counter/duration", func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		default:
			http.Error(res, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		case http.MethodPost:
			if err := req.ParseForm(); err != nil {
				http.Error(res, err.Error(), http.StatusBadRequest)
			}
			var data Configuration[time.Duration]
			d, err := parseDuration(req)
			if err != nil {
				data.Error = err.Error()
			}
			data.Value = d
			render(res, req, templates, http.StatusOK, "duration", data)
		}
	})
	mux.HandleFunc("/counter/step", func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		default:
			http.Error(res, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		case http.MethodPost:
			if err := req.ParseForm(); err != nil {
				http.Error(res, err.Error(), http.StatusBadRequest)
			}
			var data Configuration[int]
			s, err := parseStep(req)
			if err != nil {
				data.Error = err.Error()
			}
			data.Value = s
			render(res, req, templates, http.StatusOK, "step", data)
		}
	})
	mux.HandleFunc("/counter", func(res http.ResponseWriter, req *http.Request) {
		switch req.Method {
		default:
			http.Error(res, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		case http.MethodPost:
			if err := req.ParseForm(); err != nil {
				http.Error(res, err.Error(), http.StatusBadRequest)
				return
			}
			var data struct {
				Duration Configuration[time.Duration]
				Step     Configuration[int]
			}
			ok := true
			d, err := parseDuration(req)
			if err != nil {
				data.Duration.Error = err.Error()
				ok = false
			}
			data.Duration.Value = d
			s, err := parseStep(req)
			if err != nil {
				data.Step.Error = err.Error()
				ok = false
			}
			data.Step.Value = s
			if !ok {
				render(res, req, templates, http.StatusOK, "counter-configuration", data)
				return
			}
			updateStep <- s
			updateDuration <- d
			c := make(chan State)
			stateRequest <- c
			select {
			case <-req.Context().Done():
				<-c
			case state := <-c:
				data.Step.Value = state.Step
				data.Duration.Value = state.Duration
			}
			render(res, req, templates, http.StatusOK, "counter-configuration", data)
		}
	})
	mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		var data struct {
			Duration Configuration[time.Duration]
			Step     Configuration[int]
			N        int
		}
		c := make(chan State)
		stateRequest <- c
		select {
		case <-req.Context().Done():
			<-c
		case state := <-c:
			data.Step.Value = state.Step
			data.Duration.Value = state.Duration
			data.N = state.N
		}
		render(res, req, templates, http.StatusOK, "index", data)
	})
	bc := newBroadcastServer(ctx, countUpdates)
	mux.HandleFunc("/count", func(res http.ResponseWriter, req *http.Request) {
		c := bc.Subscribe()
		defer bc.CancelSubscription(c)
		ctx := req.Context()
		source, err := sse.NewEventSource(res, http.StatusOK)
		if err != nil {
			http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}
		for {
			select {
			case <-ctx.Done():
				return
			case count, open := <-c:
				if !open {
					return
				}
				_, _ = source.Send("message", strconv.Itoa(count))
			}
		}
	})

	log.Fatal(http.ListenAndServe(":"+cmp.Or(os.Getenv("PORT"), ":8080"), mux))
}

func render(res http.ResponseWriter, _ *http.Request, templates *template.Template, code int, name string, data any) {
	var buf bytes.Buffer
	if err := templates.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	res.WriteHeader(code)
	_, _ = res.Write(buf.Bytes())
}

type Configuration[T any] struct {
	Value T
	Error string
}

func parseStep(req *http.Request) (int, error) {
	s, err := strconv.Atoi(req.Form.Get("step"))
	if err != nil {
		return 0, err
	}
	if s == 0 {
		return 0, fmt.Errorf("step cannot be zero")
	}
	return s, nil
}

func parseDuration(req *http.Request) (time.Duration, error) {
	d, err := time.ParseDuration(req.Form.Get("duration"))
	if err != nil {
		return 0, err
	}
	const minimum = 10 * time.Millisecond
	if d < minimum {
		return 0, fmt.Errorf("duration must be greater than %s", minimum)
	}
	return d, nil
}
