// Package gutenberg is the library behind the gutenberg command line:
// the HTTP client, request shaping, and the typed data models for Project Gutenberg
// via the open Gutendex community API at https://gutendex.com.
//
// No authentication is required. The Client sets a real User-Agent, paces
// requests, and retries transient 429/5xx responses with exponential backoff.
package gutenberg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to Gutendex.
const DefaultUserAgent = "gutenberg/dev (+https://github.com/tamnd/gutenberg-cli)"

// ErrNotFound is returned when the Gutendex API responds with HTTP 404.
var ErrNotFound = errors.New("not found")

// Config holds constructor parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults for the Gutendex API.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://gutendex.com",
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   15 * time.Second,
	}
}

// Client talks to the Gutendex API.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	rate       time.Duration
	retries    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client configured by cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    strings.TrimRight(cfg.BaseURL, "/"),
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
	}
}

// get fetches rawURL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, ErrNotFound
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

// fetchPages collects up to limit Book records starting from firstURL,
// following the `next` pagination pointer until satisfied or exhausted.
// If limit <= 0, only the first page is fetched.
func (c *Client) fetchPages(ctx context.Context, firstURL string, limit int) ([]Book, error) {
	var out []Book
	nextURL := firstURL
	for nextURL != "" {
		var page wirePage
		if err := c.getJSON(ctx, nextURL, &page); err != nil {
			return out, err
		}
		for _, wb := range page.Results {
			out = append(out, wireBookToBook(wb))
			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
		if page.Next == nil {
			break
		}
		nextURL = *page.Next
	}
	return out, nil
}

// Search returns books matching query. Default limit is 20.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]Book, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("search", query)
	rawURL := c.baseURL + "/books/?" + params.Encode()
	return c.fetchPages(ctx, rawURL, limit)
}

// Top returns the most-downloaded books, ordered by download_count desc.
// Default limit is 20.
func (c *Client) Top(ctx context.Context, limit int) ([]Book, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	params.Set("sort", "popular")
	rawURL := c.baseURL + "/books/?" + params.Encode()
	return c.fetchPages(ctx, rawURL, limit)
}

// GetBook fetches a single book by its Gutenberg numeric ID.
// Returns ErrNotFound if the API returns HTTP 404.
func (c *Client) GetBook(ctx context.Context, id int) (Book, error) {
	rawURL := fmt.Sprintf("%s/books/%d", c.baseURL, id)
	var wb wireBook
	if err := c.getJSON(ctx, rawURL, &wb); err != nil {
		return Book{}, err
	}
	return wireBookToBook(wb), nil
}

// List returns books with optional language and topic filters.
// Either filter may be empty. Default limit is 20.
func (c *Client) List(ctx context.Context, lang, topic string, limit int) ([]Book, error) {
	if limit <= 0 {
		limit = 20
	}
	params := url.Values{}
	if lang != "" {
		params.Set("languages", lang)
	}
	if topic != "" {
		params.Set("topic", topic)
	}
	rawURL := c.baseURL + "/books/?" + params.Encode()
	return c.fetchPages(ctx, rawURL, limit)
}
