package gutenberg

import "strings"

// Author is a Gutenberg book author or translator.
type Author struct {
	Name      string `json:"name"`
	BirthYear int    `json:"birth_year,omitempty"`
	DeathYear int    `json:"death_year,omitempty"`
}

// Book is the flat record emitted for every command that returns books.
// Complex wire fields (authors list, formats map) are projected to single
// string fields for easy table / CSV rendering.
type Book struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Author        string `json:"author"`
	Languages     string `json:"languages"`
	Subjects      string `json:"subjects"`
	Bookshelves   string `json:"bookshelves"`
	DownloadCount int    `json:"download_count"`
	Copyright     string `json:"copyright"`
	EpubURL       string `json:"epub_url"`
	HTMLURL       string `json:"html_url"`
	TextURL       string `json:"text_url"`
}

// ── wire types ────────────────────────────────────────────────────────────────

type wireAuthor struct {
	Name      string `json:"name"`
	BirthYear *int   `json:"birth_year"`
	DeathYear *int   `json:"death_year"`
}

type wireBook struct {
	ID            int               `json:"id"`
	Title         string            `json:"title"`
	Authors       []wireAuthor      `json:"authors"`
	Translators   []wireAuthor      `json:"translators"`
	Subjects      []string          `json:"subjects"`
	Bookshelves   []string          `json:"bookshelves"`
	Languages     []string          `json:"languages"`
	Copyright     *bool             `json:"copyright"`
	MediaType     string            `json:"media_type"`
	Formats       map[string]string `json:"formats"`
	DownloadCount int               `json:"download_count"`
}

type wirePage struct {
	Count    int        `json:"count"`
	Next     *string    `json:"next"`
	Previous *string    `json:"previous"`
	Results  []wireBook `json:"results"`
}

// wireBookToBook converts the wire form to the flat Book record.
func wireBookToBook(w wireBook) Book {
	authorName := ""
	if len(w.Authors) > 0 {
		authorName = w.Authors[0].Name
	}

	copyright := ""
	if w.Copyright != nil {
		if *w.Copyright {
			copyright = "true"
		} else {
			copyright = "false"
		}
	}

	epubURL := w.Formats["application/epub+zip"]
	htmlURL := w.Formats["text/html"]
	textURL := ""
	for k, v := range w.Formats {
		if strings.HasPrefix(k, "text/plain") {
			textURL = v
			break
		}
	}

	return Book{
		ID:            w.ID,
		Title:         w.Title,
		Author:        authorName,
		Languages:     strings.Join(w.Languages, "; "),
		Subjects:      strings.Join(w.Subjects, "; "),
		Bookshelves:   strings.Join(w.Bookshelves, "; "),
		DownloadCount: w.DownloadCount,
		Copyright:     copyright,
		EpubURL:       epubURL,
		HTMLURL:       htmlURL,
		TextURL:       textURL,
	}
}
