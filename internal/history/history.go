package history

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

type Entry struct {
	URL   string
	Title string
}

func New() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".local", "share", "wlbrowser")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dir, "history.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS history (
			url TEXT PRIMARY KEY,
			title TEXT,
			last_visited DATETIME DEFAULT CURRENT_TIMESTAMP,
			visit_count INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Add(url, title string) {
	_, err := s.db.Exec(`
		INSERT INTO history (url, title, last_visited, visit_count)
		VALUES (?, ?, CURRENT_TIMESTAMP, 1)
		ON CONFLICT(url) DO UPDATE SET
			title = excluded.title,
			last_visited = CURRENT_TIMESTAMP,
			visit_count = visit_count + 1
	`, url, title)
	if err != nil {
		log.Printf("history Add error: %v", err)
	}
}

func (s *Store) Search(query string) []Entry {
	if query == "" {
		return nil
	}

	var b strings.Builder
	b.WriteRune('%')
	for _, r := range query {
		if r == ' ' {
			b.WriteRune('%')
		} else {
			b.WriteRune(r)
			b.WriteRune('%')
		}
	}
	fuzzy := b.String()

	rows, err := s.db.Query(`
		SELECT url, title FROM history
		WHERE url LIKE ? OR title LIKE ?
		ORDER BY visit_count DESC, last_visited DESC
		LIMIT 10
	`, fuzzy, fuzzy)
	if err != nil {
		log.Printf("history Search error: %v", err)
		return nil
	}
	defer rows.Close()

	var res []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.URL, &e.Title); err == nil {
			res = append(res, e)
		}
	}
	return res
}

func (s *Store) Delete(url string) error {
	_, err := s.db.Exec("DELETE FROM history WHERE url = ?", url)
	return err
}

func (s *Store) DeleteDomain(domain string) error {
	// We use LIKE with % to match subdomains as well if needed, 
	// but usually a simple match on the host part is safer.
	// URLs in the DB are like https://example.com/path
	pattern := "%://" + domain + "/%"
	patternExact := "%://" + domain
	_, err := s.db.Exec("DELETE FROM history WHERE url LIKE ? OR url = ?", pattern, patternExact)
	return err
}

func (s *Store) List(limit int) ([]Entry, error) {
	rows, err := s.db.Query(`
		SELECT url, title FROM history
		ORDER BY last_visited DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.URL, &e.Title); err == nil {
			res = append(res, e)
		}
	}
	return res, nil
}

func (s *Store) Close() {
	s.db.Close()
}
