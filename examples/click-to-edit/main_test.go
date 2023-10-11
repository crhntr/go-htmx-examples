package main

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/crhntr/dom"
	"github.com/crhntr/dom/domx"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/html"

	"github.com/crhntr/go-mysql-htmx/examples/click-to-edit/internal/database"
	"github.com/crhntr/go-mysql-htmx/examples/click-to-edit/internal/fakes"
)

func TestIndexLinks(t *testing.T) {
	db := new(fakes.Querier)

	db.ListContactsReturns([]database.Contact{
		{ID: 5, FirstName: "first1", LastName: "last1"},
		{ID: 6, FirstName: "first2", LastName: "last2"},
	}, nil)

	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
	document := node.(dom.Document)

	contactsLinks := document.QuerySelectorAll("ul li a[href]")
	assert.Equal(t, 2, contactsLinks.Length())
	assert.True(t, strings.Contains(contactsLinks.Item(0).TextContent(), "first1 last1"))
	assert.True(t, strings.Contains(contactsLinks.Item(1).TextContent(), "first2 last2"))
	assert.Equal(t, "/contact/5", contactsLinks.Item(0).(dom.Element).GetAttribute("href"))
	assert.Equal(t, "/contact/6", contactsLinks.Item(1).(dom.Element).GetAttribute("href"))
}

func TestIndexError(t *testing.T) {
	db := new(fakes.Querier)
	db.ListContactsReturns(nil, fmt.Errorf("banana"))
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	res := rec.Result()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
}

func TestViewContact(t *testing.T) {
	db := new(fakes.Querier)
	db.ContactWithIDReturns(database.Contact{
		ID:        5,
		FirstName: "first",
		LastName:  "last",
		Email:     "email",
	}, nil)
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodGet, "/contact/5", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func TestViewContactInvalidID(t *testing.T) {
	db := new(fakes.Querier)
	db.ContactWithIDReturns(database.Contact{
		ID:        5,
		FirstName: "first",
		LastName:  "last",
		Email:     "email",
	}, nil)
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodGet, "/contact/banana", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
}

func TestEditContact(t *testing.T) {
	db := new(fakes.Querier)
	db.ContactWithIDReturns(database.Contact{
		ID:        5,
		FirstName: "first",
		LastName:  "last",
		Email:     "email",
	}, nil)
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodGet, "/contact/5/edit", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Equal(t, http.StatusOK, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func TestViewContactNotFound(t *testing.T) {
	db := new(fakes.Querier)
	db.ContactWithIDReturns(database.Contact{}, sql.ErrNoRows)
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodGet, "/contact/5", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func TestEditContactNotFound(t *testing.T) {
	db := new(fakes.Querier)
	db.ContactWithIDReturns(database.Contact{}, sql.ErrNoRows)
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodGet, "/contact/5/edit", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func TestSubmitContact(t *testing.T) {
	db := new(fakes.Querier)
	db.UpdateContactReturns(nil)
	db.ContactWithIDReturns(database.Contact{
		ID:        5,
		FirstName: "x",
		LastName:  "y",
		Email:     "z",
	}, nil)
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodPost, "/contact/5", strings.NewReader(url.Values{
		"first-name": []string{"cara"},
		"last-name":  []string{"orange"},
		"email":      []string{"cara.orange@example.com"},
	}.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()
	assert.Equal(t, http.StatusOK, res.StatusCode)

	_, query := db.UpdateContactArgsForCall(0)
	assert.Equal(t, int64(5), query.ID)
	assert.Equal(t, "cara", query.FirstName)
	assert.Equal(t, "orange", query.LastName)
	assert.Equal(t, "cara.orange@example.com", query.Email)

	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func TestSubmitContactError(t *testing.T) {
	db := new(fakes.Querier)
	db.UpdateContactReturns(errors.New("banana"))
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodPost, "/contact/5", strings.NewReader(url.Values{
		"first-name": []string{"cara"},
		"last-name":  []string{"orange"},
		"email":      []string{"cara.orange@example.com"},
	}.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Zero(t, db.ContactWithIDCallCount())

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func TestSubmitContactUpdateFails(t *testing.T) {
	db := new(fakes.Querier)
	db.UpdateContactReturns(errors.New("banana"))
	db.ContactWithIDReturns(database.Contact{}, nil)
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodPost, "/contact/5", strings.NewReader(url.Values{
		"first-name": []string{"cara"},
		"last-name":  []string{"orange"},
		"email":      []string{"cara.orange@example.com"},
	}.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Zero(t, db.ContactWithIDCallCount())

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func TestSubmitContactGetFails(t *testing.T) {
	db := new(fakes.Querier)
	db.UpdateContactReturns(nil)
	db.ContactWithIDReturns(database.Contact{}, errors.New("banana"))
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodPost, "/contact/5", strings.NewReader(url.Values{
		"first-name": []string{"cara"},
		"last-name":  []string{"orange"},
		"email":      []string{"cara.orange@example.com"},
	}.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func TestSubmitContactParseFails(t *testing.T) {
	db := new(fakes.Querier)
	db.UpdateContactReturns(nil)
	db.ContactWithIDReturns(database.Contact{}, nil)
	mux := newServer(db).routes()

	req := httptest.NewRequest(http.MethodPost, "/contact/5", iotest.ErrReader(errors.New("banana")))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	res := rec.Result()

	assert.Zero(t, db.UpdateContactCallCount())

	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	node := parseResponseNode(t, res)
	assert.IsType(t, &domx.Document{}, node)
}

func Test_write_full_page_missing_page(t *testing.T) {
	db := new(fakes.Querier)
	server := newServer(db)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.writePage(rec, req, "banana", http.StatusOK, struct{}{})
	res := rec.Result()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
}

func Test_write_page(t *testing.T) {
	db := new(fakes.Querier)
	server := newServer(db)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("hx-target", "view")
	rec := httptest.NewRecorder()
	server.writePage(rec, req, "banana", http.StatusOK, struct{}{})
	res := rec.Result()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
}

func Test_must(t *testing.T) {
	assert.Panics(t, func() {
		must(0, errors.New("banana"))
	})
	assert.NotPanics(t, func() {
		assert.Equal(t, 5, must(5, nil))
	})
}

func parseResponseNode(t *testing.T, res *http.Response) dom.Node {
	t.Helper()
	body, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	node, err := html.Parse(bytes.NewReader(body))
	assert.NoError(t, err)
	return domx.NewNode(node)
}
