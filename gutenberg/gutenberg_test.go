package gutenberg_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/gutenberg-cli/gutenberg"
)

// pageJSON builds a minimal Gutendex page JSON with the given books.
func pageJSON(books string, next string) string {
	nextField := "null"
	if next != "" {
		nextField = `"` + next + `"`
	}
	return `{"count":100,"next":` + nextField + `,"previous":null,"results":[` + books + `]}`
}

const sampleBook = `{
	"id": 1342,
	"title": "Pride and Prejudice",
	"authors": [{"name": "Austen, Jane", "birth_year": 1775, "death_year": 1817}],
	"translators": [],
	"subjects": ["Domestic fiction"],
	"bookshelves": ["Best Books Ever Listings"],
	"languages": ["en"],
	"copyright": false,
	"media_type": "Text",
	"formats": {
		"application/epub+zip": "https://www.gutenberg.org/ebooks/1342.epub.images",
		"text/html": "https://www.gutenberg.org/files/1342/1342-h/1342-h.htm",
		"text/plain; charset=utf-8": "https://www.gutenberg.org/files/1342/1342-0.txt"
	},
	"download_count": 90000
}`

func newTestClient(ts *httptest.Server) *gutenberg.Client {
	cfg := gutenberg.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return gutenberg.NewClient(cfg)
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "search=austen") {
			t.Errorf("expected search=austen in query, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(pageJSON(sampleBook, "")))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	books, err := c.Search(context.Background(), "austen", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) == 0 {
		t.Fatal("expected at least one book")
	}
	if books[0].Title != "Pride and Prejudice" {
		t.Errorf("first title = %q", books[0].Title)
	}
	if books[0].Author != "Austen, Jane" {
		t.Errorf("author = %q", books[0].Author)
	}
}

func TestTop(t *testing.T) {
	var gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.RawQuery
		_, _ = w.Write([]byte(pageJSON(sampleBook, "")))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.Top(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotURL, "sort=popular") {
		t.Errorf("expected sort=popular in query, got %q", gotURL)
	}
}

func TestGetBook(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(sampleBook))
		}))
		defer srv.Close()

		c := newTestClient(srv)
		book, err := c.GetBook(context.Background(), 1342)
		if err != nil {
			t.Fatal(err)
		}
		if book.ID != 1342 {
			t.Errorf("book ID = %d, want 1342", book.ID)
		}
		if book.EpubURL == "" {
			t.Error("epub_url should not be empty")
		}
		if book.TextURL == "" {
			t.Error("text_url should not be empty")
		}
	})

	t.Run("not_found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer srv.Close()

		c := newTestClient(srv)
		_, err := c.GetBook(context.Background(), 999999999)
		if !errors.Is(err, gutenberg.ErrNotFound) {
			t.Errorf("got %v, want ErrNotFound", err)
		}
	})
}

func TestList(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(pageJSON(sampleBook, "")))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	_, err := c.List(context.Background(), "en", "science", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotQuery, "languages=en") {
		t.Errorf("expected languages=en in query, got %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "topic=science") {
		t.Errorf("expected topic=science in query, got %q", gotQuery)
	}
}

func TestPagination(t *testing.T) {
	const book2 = `{
		"id": 11,
		"title": "Alice in Wonderland",
		"authors": [{"name": "Carroll, Lewis", "birth_year": 1832, "death_year": 1898}],
		"translators": [],
		"subjects": [],
		"bookshelves": [],
		"languages": ["en"],
		"copyright": false,
		"media_type": "Text",
		"formats": {},
		"download_count": 84000
	}`

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.URL.Path == "/page2" {
			_, _ = w.Write([]byte(pageJSON(book2, "")))
		} else {
			_, _ = w.Write([]byte(pageJSON(sampleBook, "")))
		}
	}))
	defer srv.Close()

	// Patch: the server's /books/ endpoint returns next pointing to /page2.
	page1 := `{"count":2,"next":"` + srv.URL + `/page2","previous":null,"results":[` + sampleBook + `]}`
	var requestNum int
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestNum++
		if requestNum == 1 {
			_, _ = w.Write([]byte(page1))
		} else {
			_, _ = w.Write([]byte(pageJSON(book2, "")))
		}
	}))
	defer srv2.Close()

	cfg := gutenberg.DefaultConfig()
	cfg.BaseURL = srv2.URL
	cfg.Rate = 0
	c := gutenberg.NewClient(cfg)

	books, err := c.Search(context.Background(), "q", 40)
	if err != nil {
		t.Fatal(err)
	}
	if len(books) < 2 {
		t.Errorf("expected at least 2 books across 2 pages, got %d", len(books))
	}
}

func TestRateZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(pageJSON(sampleBook, "")))
	}))
	defer srv.Close()

	c := newTestClient(srv)
	start := time.Now()
	_, _ = c.Top(context.Background(), 5)
	_, _ = c.Top(context.Background(), 5)
	if time.Since(start) > 50*time.Millisecond {
		t.Error("Rate=0 still caused a sleep")
	}
}

func TestRetryOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(pageJSON(sampleBook, "")))
	}))
	defer srv.Close()

	cfg := gutenberg.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := gutenberg.NewClient(cfg)

	_, err := c.Top(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}
