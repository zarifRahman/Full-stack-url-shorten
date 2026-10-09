package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const schema = `
CREATE TABLE IF NOT EXISTS links (
	id         BIGSERIAL PRIMARY KEY,
	code       TEXT UNIQUE NOT NULL,
	url        TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS clicks (
	id         BIGSERIAL PRIMARY KEY,
	link_id    BIGINT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
	referer    TEXT,
	user_agent TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_clicks_link ON clicks(link_id);
`

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var (
	db      *pgxpool.Pool
	baseURL string
)

// main wires everything: reads config from env, connects to Postgres (with
// retry), creates the schema, registers routes, and starts the HTTP server.
func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL not set")
	}
	baseURL = os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	ctx := context.Background()
	var err error
	// ponytail: 10x retry — Postgres may still be booting when backend starts.
	for i := 0; i < 10; i++ {
		db, err = pgxpool.New(ctx, dsn)
		if err == nil {
			err = db.Ping(ctx)
		}
		if err == nil {
			break
		}
		log.Printf("waiting for postgres (%d/10): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("cannot connect to postgres: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(ctx, schema); err != nil {
		log.Fatalf("schema init failed: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("POST /api/links", createLink)
	mux.HandleFunc("GET /api/links", listLinks)
	mux.HandleFunc("GET /api/links/{code}/stats", linkStats)
	mux.HandleFunc("GET /{code}", redirect)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", withCORS(mux)))
}

// withCORS wraps a handler to allow cross-origin requests from the browser
// (frontend on :3000 calls the API on :8080) and answers CORS preflight.
func withCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// shortCode returns a random n-character base62 string, used as the public
// code in a short link (e.g. "KItHi3"). Backed by crypto/rand.
func shortCode(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// normalizeURL cleans user input into a storable URL: trims it, prepends
// https:// when no scheme is present, and errors on empty/invalid host.
func normalizeURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty url")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return "", errors.New("invalid url")
	}
	return u.String(), nil
}

// writeJSON sets the JSON content type, writes the status code, and encodes v
// as the response body. Shared by every handler.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// health is a liveness probe — returns {"status":"ok"} so Docker/monitors can
// tell the server is up.
func health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// createLink handles POST /api/links: validates the submitted URL, generates
// a unique short code (retrying on the rare collision), inserts the row, and
// returns {code, url, short_url}.
func createLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	target, err := normalizeURL(body.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	ctx := r.Context()
	var code string
	// Retry on unique-violation (rare collision on random code).
	for attempt := 0; attempt < 5; attempt++ {
		code = shortCode(6)
		_, err = db.Exec(ctx, "INSERT INTO links (code, url) VALUES ($1, $2)", code, target)
		if err == nil {
			break
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			continue // collision, try new code
		}
		log.Printf("insert link failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not generate code"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"code":      code,
		"url":       target,
		"short_url": baseURL + "/" + code,
	})
}

// listLinks handles GET /api/links: returns every link with its total click
// count (LEFT JOIN so links with zero clicks still appear), newest first.
func listLinks(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(r.Context(), `
		SELECT l.code, l.url, l.created_at, count(c.id)
		FROM links l
		LEFT JOIN clicks c ON c.link_id = l.id
		GROUP BY l.id
		ORDER BY l.created_at DESC`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	defer rows.Close()

	type link struct {
		Code      string    `json:"code"`
		URL       string    `json:"url"`
		ShortURL  string    `json:"short_url"`
		Clicks    int64     `json:"clicks"`
		CreatedAt time.Time `json:"created_at"`
	}
	out := []link{}
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.Code, &l.URL, &l.CreatedAt, &l.Clicks); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "scan error"})
			return
		}
		l.ShortURL = baseURL + "/" + l.Code
		out = append(out, l)
	}
	writeJSON(w, http.StatusOK, out)
}

// linkStats handles GET /api/links/{code}/stats: resolves the code to a link,
// then returns total clicks, per-day counts for the last 30 days (for the
// chart), and the 20 most recent clicks. 404 if the code is unknown.
func linkStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	ctx := r.Context()

	var linkID int64
	var target string
	err := db.QueryRow(ctx, "SELECT id, url FROM links WHERE code=$1", code).Scan(&linkID, &target)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}

	var total int64
	db.QueryRow(ctx, "SELECT count(*) FROM clicks WHERE link_id=$1", linkID).Scan(&total)

	type day struct {
		Date  string `json:"date"`
		Count int64  `json:"count"`
	}
	daily := []day{}
	drows, err := db.Query(ctx, `
		SELECT to_char(date_trunc('day', created_at), 'YYYY-MM-DD'), count(*)
		FROM clicks
		WHERE link_id=$1 AND created_at > now() - interval '30 days'
		GROUP BY 1 ORDER BY 1`, linkID)
	if err == nil {
		defer drows.Close()
		for drows.Next() {
			var d day
			drows.Scan(&d.Date, &d.Count)
			daily = append(daily, d)
		}
	}

	type click struct {
		CreatedAt time.Time `json:"created_at"`
		Referer   string    `json:"referer"`
		UserAgent string    `json:"user_agent"`
	}
	recent := []click{}
	rrows, err := db.Query(ctx, `
		SELECT created_at, coalesce(referer,''), coalesce(user_agent,'')
		FROM clicks WHERE link_id=$1 ORDER BY created_at DESC LIMIT 20`, linkID)
	if err == nil {
		defer rrows.Close()
		for rrows.Next() {
			var c click
			rrows.Scan(&c.CreatedAt, &c.Referer, &c.UserAgent)
			recent = append(recent, c)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"code":   code,
		"url":    target,
		"total":  total,
		"daily":  daily,
		"recent": recent,
	})
}

// redirect handles GET /{code}: looks up the original URL, logs the click in
// the background, and 302-redirects the visitor. 404 if the code is unknown.
func redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	var linkID int64
	var target string
	err := db.QueryRow(r.Context(), "SELECT id, url FROM links WHERE code=$1", code).Scan(&linkID, &target)
	if errors.Is(err, pgx.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	// ponytail: fire-and-forget click log — a dropped click on crash is acceptable.
	referer, ua := r.Referer(), r.UserAgent()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db.Exec(ctx, "INSERT INTO clicks (link_id, referer, user_agent) VALUES ($1,$2,$3)", linkID, referer, ua)
	}()

	http.Redirect(w, r, target, http.StatusFound)
}
