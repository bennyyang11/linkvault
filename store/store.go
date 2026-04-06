package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

type Bookmark struct {
	ID          int       `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	FaviconURL  string    `json:"favicon_url"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Tag struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Collection struct {
	ID          int        `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	IsPublic    bool       `json:"is_public"`
	ShareCode   string     `json:"share_code,omitempty"`
	Bookmarks   []Bookmark `json:"bookmarks,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type Store struct {
	db          *sql.DB
	searchCount atomic.Int64
	startTime   time.Time
}

func New(connStr string) (*Store, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	for i := 0; i < 15; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		log.Printf("Waiting for database... (%d/15)", i+1)
		time.Sleep(2 * time.Second)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database not reachable after 30s: %w", err)
	}

	s := &Store{db: db, startTime: time.Now()}
	if err := s.runMigrations(); err != nil {
		return nil, fmt.Errorf("migrations failed: %w", err)
	}
	log.Println("Database connected and migrations complete")
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Ping() error { return s.db.Ping() }

func (s *Store) runMigrations() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS bookmarks (
			id          SERIAL PRIMARY KEY,
			url         TEXT NOT NULL,
			title       TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			favicon_url TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS tags (
			id   SERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE
		)`,
		`CREATE TABLE IF NOT EXISTS bookmark_tags (
			bookmark_id INT REFERENCES bookmarks(id) ON DELETE CASCADE,
			tag_id      INT REFERENCES tags(id) ON DELETE CASCADE,
			PRIMARY KEY (bookmark_id, tag_id)
		)`,
		`CREATE TABLE IF NOT EXISTS collections (
			id          SERIAL PRIMARY KEY,
			name        TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			is_public   BOOLEAN NOT NULL DEFAULT false,
			share_code  TEXT UNIQUE,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS collection_bookmarks (
			collection_id INT REFERENCES collections(id) ON DELETE CASCADE,
			bookmark_id   INT REFERENCES bookmarks(id) ON DELETE CASCADE,
			position      INT NOT NULL DEFAULT 0,
			PRIMARY KEY (collection_id, bookmark_id)
		)`,
	}
	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return err
		}
	}
	return nil
}

// --- Bookmarks ---

func (s *Store) CreateBookmark(url, title, description, faviconURL string, tagNames []string) (*Bookmark, error) {
	var b Bookmark
	err := s.db.QueryRow(
		`INSERT INTO bookmarks (url, title, description, favicon_url) VALUES ($1,$2,$3,$4)
		 RETURNING id, url, title, description, favicon_url, created_at, updated_at`,
		url, title, description, faviconURL,
	).Scan(&b.ID, &b.URL, &b.Title, &b.Description, &b.FaviconURL, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}

	b.Tags = []string{}
	for _, name := range tagNames {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			continue
		}
		tagID, err := s.ensureTag(name)
		if err != nil {
			continue
		}
		s.db.Exec(`INSERT INTO bookmark_tags (bookmark_id, tag_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, b.ID, tagID)
		b.Tags = append(b.Tags, name)
	}
	return &b, nil
}

func (s *Store) ListBookmarks(tag, query string) ([]Bookmark, error) {
	baseQuery := `SELECT b.id, b.url, b.title, b.description, b.favicon_url, b.created_at, b.updated_at,
		COALESCE(string_agg(t.name, ',' ORDER BY t.name), '') as tag_list
		FROM bookmarks b
		LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
		LEFT JOIN tags t ON bt.tag_id = t.id`

	var where []string
	var args []interface{}
	argN := 1

	if tag != "" {
		where = append(where, fmt.Sprintf(
			`b.id IN (SELECT bt2.bookmark_id FROM bookmark_tags bt2 JOIN tags t2 ON bt2.tag_id=t2.id WHERE t2.name=$%d)`, argN))
		args = append(args, strings.ToLower(tag))
		argN++
	}
	if query != "" {
		s.searchCount.Add(1)
		where = append(where, fmt.Sprintf(
			`(b.title ILIKE '%%' || $%d || '%%' OR b.url ILIKE '%%' || $%d || '%%' OR b.description ILIKE '%%' || $%d || '%%')`,
			argN, argN, argN))
		args = append(args, query)
		argN++
	}

	if len(where) > 0 {
		baseQuery += " WHERE " + strings.Join(where, " AND ")
	}
	baseQuery += " GROUP BY b.id ORDER BY b.created_at DESC"

	rows, err := s.db.Query(baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []Bookmark
	for rows.Next() {
		var b Bookmark
		var tagList string
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.Description, &b.FaviconURL,
			&b.CreatedAt, &b.UpdatedAt, &tagList); err != nil {
			return nil, err
		}
		if tagList != "" {
			b.Tags = strings.Split(tagList, ",")
		} else {
			b.Tags = []string{}
		}
		bookmarks = append(bookmarks, b)
	}
	if bookmarks == nil {
		bookmarks = []Bookmark{}
	}
	return bookmarks, nil
}

func (s *Store) DeleteBookmark(id int) error {
	res, err := s.db.Exec(`DELETE FROM bookmarks WHERE id=$1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("bookmark not found")
	}
	return nil
}

// --- Tags ---

func (s *Store) ensureTag(name string) (int, error) {
	var id int
	err := s.db.QueryRow(`INSERT INTO tags (name) VALUES ($1) ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name RETURNING id`, name).Scan(&id)
	return id, err
}

func (s *Store) ListTags() ([]Tag, error) {
	rows, err := s.db.Query(
		`SELECT t.id, t.name, COUNT(bt.bookmark_id) as cnt
		 FROM tags t LEFT JOIN bookmark_tags bt ON t.id = bt.tag_id
		 GROUP BY t.id ORDER BY cnt DESC, t.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Count); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	if tags == nil {
		tags = []Tag{}
	}
	return tags, nil
}

// --- Collections ---

func (s *Store) CreateCollection(name, description string) (*Collection, error) {
	var c Collection
	err := s.db.QueryRow(
		`INSERT INTO collections (name, description) VALUES ($1,$2) RETURNING id, name, description, is_public, share_code, created_at`,
		name, description,
	).Scan(&c.ID, &c.Name, &c.Description, &c.IsPublic, &sql.NullString{}, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	c.Bookmarks = []Bookmark{}
	return &c, nil
}

func (s *Store) ListCollections() ([]Collection, error) {
	rows, err := s.db.Query(
		`SELECT c.id, c.name, c.description, c.is_public, COALESCE(c.share_code,''), c.created_at,
		 COUNT(cb.bookmark_id) as bookmark_count
		 FROM collections c LEFT JOIN collection_bookmarks cb ON c.id = cb.collection_id
		 GROUP BY c.id ORDER BY c.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var collections []Collection
	for rows.Next() {
		var c Collection
		var count int
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.IsPublic, &c.ShareCode, &c.CreatedAt, &count); err != nil {
			return nil, err
		}
		collections = append(collections, c)
	}
	if collections == nil {
		collections = []Collection{}
	}
	return collections, nil
}

func (s *Store) GetCollection(id int) (*Collection, error) {
	var c Collection
	err := s.db.QueryRow(
		`SELECT id, name, description, is_public, COALESCE(share_code,''), created_at FROM collections WHERE id=$1`, id,
	).Scan(&c.ID, &c.Name, &c.Description, &c.IsPublic, &c.ShareCode, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	bookmarks, err := s.collectionBookmarks(id)
	if err != nil {
		return nil, err
	}
	c.Bookmarks = bookmarks
	return &c, nil
}

func (s *Store) GetCollectionByShareCode(code string) (*Collection, error) {
	var c Collection
	err := s.db.QueryRow(
		`SELECT id, name, description, is_public, share_code, created_at FROM collections WHERE share_code=$1 AND is_public=true`, code,
	).Scan(&c.ID, &c.Name, &c.Description, &c.IsPublic, &c.ShareCode, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	bookmarks, err := s.collectionBookmarks(c.ID)
	if err != nil {
		return nil, err
	}
	c.Bookmarks = bookmarks
	return &c, nil
}

func (s *Store) ToggleCollectionPublic(id int) (*Collection, error) {
	var isPublic bool
	err := s.db.QueryRow(`SELECT is_public FROM collections WHERE id=$1`, id).Scan(&isPublic)
	if err != nil {
		return nil, err
	}

	newPublic := !isPublic
	var shareCode sql.NullString
	if newPublic {
		code := generateShareCode()
		shareCode = sql.NullString{String: code, Valid: true}
	}

	_, err = s.db.Exec(`UPDATE collections SET is_public=$1, share_code=$2 WHERE id=$3`, newPublic, shareCode, id)
	if err != nil {
		return nil, err
	}
	return s.GetCollection(id)
}

func (s *Store) AddBookmarkToCollection(collectionID, bookmarkID int) error {
	_, err := s.db.Exec(
		`INSERT INTO collection_bookmarks (collection_id, bookmark_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		collectionID, bookmarkID)
	return err
}

func (s *Store) RemoveBookmarkFromCollection(collectionID, bookmarkID int) error {
	_, err := s.db.Exec(`DELETE FROM collection_bookmarks WHERE collection_id=$1 AND bookmark_id=$2`, collectionID, bookmarkID)
	return err
}

func (s *Store) collectionBookmarks(collectionID int) ([]Bookmark, error) {
	rows, err := s.db.Query(
		`SELECT b.id, b.url, b.title, b.description, b.favicon_url, b.created_at, b.updated_at,
		 COALESCE(string_agg(t.name, ',' ORDER BY t.name), '') as tag_list
		 FROM bookmarks b
		 JOIN collection_bookmarks cb ON b.id = cb.bookmark_id
		 LEFT JOIN bookmark_tags bt ON b.id = bt.bookmark_id
		 LEFT JOIN tags t ON bt.tag_id = t.id
		 WHERE cb.collection_id = $1
		 GROUP BY b.id, cb.position ORDER BY cb.position, b.created_at DESC`, collectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookmarks []Bookmark
	for rows.Next() {
		var b Bookmark
		var tagList string
		if err := rows.Scan(&b.ID, &b.URL, &b.Title, &b.Description, &b.FaviconURL,
			&b.CreatedAt, &b.UpdatedAt, &tagList); err != nil {
			return nil, err
		}
		if tagList != "" {
			b.Tags = strings.Split(tagList, ",")
		} else {
			b.Tags = []string{}
		}
		bookmarks = append(bookmarks, b)
	}
	if bookmarks == nil {
		bookmarks = []Bookmark{}
	}
	return bookmarks, nil
}

// --- Metrics ---

func (s *Store) CountBookmarks() int {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM bookmarks`).Scan(&n)
	return n
}

func (s *Store) BookmarksAddedToday() int {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM bookmarks WHERE created_at >= CURRENT_DATE`).Scan(&n)
	return n
}

func (s *Store) CountCollections() int {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM collections`).Scan(&n)
	return n
}

func (s *Store) CountTags() int {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM tags`).Scan(&n)
	return n
}

func (s *Store) StorageUsedMB() float64 {
	var bytes int64
	s.db.QueryRow(`SELECT pg_database_size(current_database())`).Scan(&bytes)
	return float64(bytes) / (1024 * 1024)
}

func (s *Store) SearchesToday() int64 {
	return s.searchCount.Load()
}

func (s *Store) UptimeHours() float64 {
	return time.Since(s.startTime).Hours()
}

// --- Helpers ---

func generateShareCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
