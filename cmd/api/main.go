package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	httpapi "bookpulse/internal/http"
	"bookpulse/internal/repo"
	"bookpulse/internal/service/auth"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UpdateProfileRequest struct {
	Name string `json:"name"`
}

type UpdatePasswordRequest struct {
	Password string `json:"password"`
}
type CreateCollectionRequest struct {
	Name    string `json:"name"`
	BookIDs []int  `json:"bookIds"`
}
type CreateReviewRequest struct {
	Rating int    `json:"rating"`
	Text   string `json:"text"`
}

type ReviewDto struct {
	ID        int    `json:"id"`
	UserName  string `json:"userName"`
	CreatedAt string `json:"createdAt"`
	Rating    int    `json:"rating"`
	Text      string `json:"text"`
}

type MyBookDTO struct {
	BookID      int      `json:"bookId"`
	GoogleID    string   `json:"googleId"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	CoverURL    string   `json:"coverUrl"`
	Status      string   `json:"status"`
	Collections []string `json:"collections"`
}
type MyCollectionDTO struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}
type UpdateStatusRequest struct {
	GoogleID string `json:"googleId"`
	BookID   int    `json:"bookId"`
	Status   string `json:"status"`
}

type AddBooksToCollectionRequest struct {
	CollectionID int      `json:"collectionId"`
	GoogleIDs    []string `json:"googleIds"`
}

type CollectionMiniDTO struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ChangeStatusRequest struct {
	BookID int    `json:"bookId"`
	Status string `json:"status"`
}

type AddToCollectionRequest struct {
	CollectionID int `json:"collectionId"`
	BookID       int `json:"bookId"`
}

type AddMyBookRequest struct {
	GoogleID string `json:"googleId"`
	ID       string `json:"id"`

	Title       string `json:"title"`
	Author      string `json:"author"`
	CoverURL    string `json:"coverUrl"`
	Description string `json:"description"`

	Categories    []string `json:"categories"`
	PublishedYear int      `json:"publishedYear"`
	PageCount     int      `json:"pageCount"`
	Maturity      string   `json:"maturity"`
	Status        string   `json:"status"`
}

type GenreStatDto struct {
	Genre string `json:"genre"`
	Cnt   int    `json:"cnt"`
}

type MonthStatDto struct {
	Month string `json:"month"`
	Cnt   int    `json:"cnt"`
}

type StatsResponse struct {
	Genres []GenreStatDto `json:"genres"`
	Months []MonthStatDto `json:"months"`
}

func main() {

	dsn := "host=127.0.0.1 port=5433 user=bookpulse password=bookpulse dbname=bookpulse sslmode=disable"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbpool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}
	defer dbpool.Close()

	if err := dbpool.Ping(ctx); err != nil {
		log.Fatal("БД не отвечает:", err)
	}
	_, err = dbpool.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	`)
	if err != nil {
		log.Fatal("Не удалось создать таблицу users:", err)
	}

	log.Println("Успешное подключение к PostgreSQL")

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})

	log.Println("Сервер запущен на http://localhost:8080")
	jwtSecret := "dev_secret_change_me"
	jwt := auth.NewJWT(jwtSecret)
	userRepo := repo.NewUserRepoPGX(dbpool)
	authSvc := auth.NewServicePGX(userRepo, jwt)

	http.HandleFunc("/api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if body.Name == "" {
			body.Name = "User"
		}

		resp, err := authSvc.Register(r.Context(), body.Email, body.Password, body.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		writeJSON(w, resp)
	})

	http.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		resp, err := authSvc.Login(r.Context(), body.Email, body.Password)
		if err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		writeJSON(w, resp)
	})

	http.HandleFunc("/api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := mustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		me, err := authSvc.Me(r.Context(), userID)
		if err != nil {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		writeJSON(w, me)
	})

	// ✅ UPDATE NAME (только name)
	http.HandleFunc("/api/me/profile", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := mustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var body UpdateProfileRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(body.Name)
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}

		_, err := dbpool.Exec(r.Context(),
			`UPDATE users SET name=$2 WHERE id=$1`,
			userID, name,
		)
		if err != nil {
			http.Error(w, "DB update error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"ok": true, "name": name})
	})

	// ✅ UPDATE PASSWORD (без подтверждения старого)
	http.HandleFunc("/api/me/password", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := mustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var body UpdatePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		pass := strings.TrimSpace(body.Password)
		if len(pass) < 6 {
			http.Error(w, "password must be at least 6 chars", http.StatusBadRequest)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "hash error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = dbpool.Exec(r.Context(),
			`UPDATE users SET password_hash=$2 WHERE id=$1`,
			userID, string(hash),
		)
		if err != nil {
			http.Error(w, "DB update error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"ok": true})
	})

	http.HandleFunc("/api/me/books", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		userID, ok := mustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {

		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			rows, err := dbpool.Query(r.Context(), `
			SELECT
			b.id,
			b.google_id,
			b.title,
			b.author,
			b.cover_url,
			ub.status,
			COALESCE(string_agg(c.name, ',' ORDER BY c.name), '') AS collections_csv
			FROM user_books ub
			JOIN books b ON b.id = ub.book_id
			LEFT JOIN collection_books cb
			  ON cb.user_id = ub.user_id AND cb.book_id = ub.book_id
			LEFT JOIN collections c
			  ON c.id = cb.collection_id AND c.user_id = cb.user_id
			WHERE ub.user_id = $1
			GROUP BY b.id, b.title, b.author, ub.status
			ORDER BY b.title;
		`, userID)
			if err != nil {
				http.Error(w, "DB query error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			result := make([]MyBookDTO, 0, 16)
			for rows.Next() {
				var dto MyBookDTO
				var collectionsCSV string

				if err := rows.Scan(&dto.BookID, &dto.GoogleID, &dto.Title, &dto.Author, &dto.CoverURL, &dto.Status, &collectionsCSV); err != nil {
					http.Error(w, "DB scan error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				dto.Collections = splitCSV(collectionsCSV)
				result = append(result, dto)
			}
			if err := rows.Err(); err != nil {
				http.Error(w, "DB rows error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			writeJSON(w, result)
			return

		case http.MethodPost:
			var body AddMyBookRequest

			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			if body.GoogleID == "" {
				body.GoogleID = body.ID
			}

			if body.GoogleID == "" || body.Title == "" {
				http.Error(w, "id and title required", http.StatusBadRequest)
				return
			}
			if body.Status == "" {
				body.Status = "planned"
			}

			var bookID int
			err := dbpool.QueryRow(r.Context(), `
			INSERT INTO books (google_id, title, author, cover_url, description, published_year, page_count, age_rating)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (google_id) DO UPDATE SET
			  title = EXCLUDED.title,
			  author = EXCLUDED.author,
			  cover_url = EXCLUDED.cover_url,
			  description = EXCLUDED.description,
			  published_year = EXCLUDED.published_year,
			  page_count = EXCLUDED.page_count,
			  age_rating = EXCLUDED.age_rating
			RETURNING id;
		`,
				body.GoogleID,
				body.Title,
				body.Author,
				body.CoverURL,
				body.Description,
				nullIfZero(body.PublishedYear),
				nullIfZero(body.PageCount),
				maturityToAge(body.Maturity),
			).Scan(&bookID)
			if err != nil {
				http.Error(w, "DB insert book error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			for _, raw := range body.Categories {
  parts := strings.Split(raw, "/")

  for i := 0; i < len(parts) && i < 2; i++ {
    g := strings.TrimSpace(parts[i])
    if g == "" || g == "General" { continue }

					var genreID int
					err := dbpool.QueryRow(r.Context(), `
      INSERT INTO genres (name)
      VALUES ($1)
      ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
      RETURNING id;
    `, g).Scan(&genreID)
					if err != nil {
						http.Error(w, "DB insert genre error: "+err.Error(), http.StatusInternalServerError)
						return
					}

					_, err = dbpool.Exec(r.Context(), `
      INSERT INTO book_genres (book_id, genre_id)
      VALUES ($1,$2)
      ON CONFLICT DO NOTHING;
    `, bookID, genreID)
					if err != nil {
						http.Error(w, "DB insert book_genres error: "+err.Error(), http.StatusInternalServerError)
						return
					}
				}
			}

			_, err = dbpool.Exec(r.Context(), `
  INSERT INTO user_books (user_id, book_id, status)
  VALUES ($1,$2,$3)
  ON CONFLICT (user_id, book_id) DO UPDATE SET status = EXCLUDED.status;
`, userID, bookID, body.Status)

			_, err = dbpool.Exec(r.Context(), `
			INSERT INTO user_books (user_id, book_id, status)
			VALUES ($1,$2,$3)
			ON CONFLICT (user_id, book_id) DO UPDATE SET status = EXCLUDED.status;
		`, userID, bookID, body.Status)
			if err != nil {
				http.Error(w, "DB insert user_books error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			writeJSON(w, map[string]any{
				"ok":       true,
				"bookId":   bookID,
				"googleId": body.GoogleID,
			})
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	http.HandleFunc("/api/me/books/status", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		userID, ok := mustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body UpdateStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		if body.Status == "" {
			http.Error(w, "status required", http.StatusBadRequest)
			return
		}
		if !isValidStatus(body.Status) {
			http.Error(w, "invalid status", http.StatusBadRequest)
			return
		}

		bookID := body.BookID
		if bookID == 0 {
			if body.GoogleID == "" {
				http.Error(w, "googleId or bookId required", http.StatusBadRequest)
				return
			}
			if err := dbpool.QueryRow(r.Context(), `SELECT id FROM books WHERE google_id=$1`, body.GoogleID).Scan(&bookID); err != nil {
				http.Error(w, "book not found", http.StatusBadRequest)
				return
			}
		}

		cmd, err := dbpool.Exec(r.Context(), `
		UPDATE user_books SET status=$3
		WHERE user_id=$1 AND book_id=$2
	`, userID, bookID, body.Status)
		if err != nil {
			http.Error(w, "DB update error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if cmd.RowsAffected() == 0 {
			http.Error(w, "book not in user's library", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		writeJSON(w, map[string]any{"ok": true})
	})

	http.HandleFunc("/api/me/collections", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		userID, ok := mustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			rows, err := dbpool.Query(r.Context(), `
			SELECT c.id, c.name, COUNT(cb.book_id) AS cnt
			FROM collections c
			LEFT JOIN collection_books cb
			  ON cb.user_id = c.user_id AND cb.collection_id = c.id
			WHERE c.user_id = $1
			GROUP BY c.id, c.name
			ORDER BY c.name;
		`, userID)
			if err != nil {
				http.Error(w, "DB query error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			out := make([]MyCollectionDTO, 0, 16)
			for rows.Next() {
				var dto MyCollectionDTO
				if err := rows.Scan(&dto.ID, &dto.Name, &dto.Count); err != nil {
					http.Error(w, "DB scan error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				out = append(out, dto)
			}
			if err := rows.Err(); err != nil {
				http.Error(w, "DB rows error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			writeJSON(w, out)
			return

		case http.MethodPost:
			var body CreateCollectionRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}

			name := body.Name
			if name == "" {
				http.Error(w, "name required", http.StatusBadRequest)
				return
			}
			if len(body.BookIDs) == 0 {
				http.Error(w, "bookIds required", http.StatusBadRequest)
				return
			}

			var collectionID int
			err := dbpool.QueryRow(r.Context(), `
		INSERT INTO collections (user_id, name)
		VALUES ($1,$2)
		ON CONFLICT (user_id, name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id;
	`, userID, name).Scan(&collectionID)
			if err != nil {
				http.Error(w, "DB insert collection error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			for _, bookID := range body.BookIDs {
				if bookID == 0 {
					continue
				}

				var inLib bool
				if err := dbpool.QueryRow(r.Context(), `
			SELECT EXISTS(SELECT 1 FROM user_books WHERE user_id=$1 AND book_id=$2)
		`, userID, bookID).Scan(&inLib); err != nil {
					http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				if !inLib {
					http.Error(w, "book not in user's library", http.StatusBadRequest)
					return
				}

				_, err := dbpool.Exec(r.Context(), `
			INSERT INTO collection_books (user_id, collection_id, book_id)
			VALUES ($1,$2,$3)
			ON CONFLICT DO NOTHING
		`, userID, collectionID, bookID)
				if err != nil {
					http.Error(w, "DB insert collection_books error: "+err.Error(), http.StatusInternalServerError)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			writeJSON(w, map[string]any{
				"ok":           true,
				"collectionId": collectionID,
			})
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	http.HandleFunc("/api/me/collections/add-books", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		userID, ok := mustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var body AddBooksToCollectionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if body.CollectionID == 0 || len(body.GoogleIDs) == 0 {
			http.Error(w, "collectionId and googleIds required", http.StatusBadRequest)
			return
		}

		var exists bool
		if err := dbpool.QueryRow(r.Context(), `
		SELECT EXISTS(SELECT 1 FROM collections WHERE id=$1 AND user_id=$2)
	`, body.CollectionID, userID).Scan(&exists); err != nil {
			http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if !exists {
			http.Error(w, "collection not found", http.StatusBadRequest)
			return
		}

		for _, gid := range body.GoogleIDs {
			if gid == "" {
				continue
			}

			var bookID int
			err := dbpool.QueryRow(r.Context(), `SELECT id FROM books WHERE google_id=$1`, gid).Scan(&bookID)
			if err != nil {
				http.Error(w, "book not found: "+gid, http.StatusBadRequest)
				return
			}

			var inLib bool
			if err := dbpool.QueryRow(r.Context(), `
			SELECT EXISTS(SELECT 1 FROM user_books WHERE user_id=$1 AND book_id=$2)
		`, userID, bookID).Scan(&inLib); err != nil {
				http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if !inLib {
				http.Error(w, "book not in user's library", http.StatusBadRequest)
				return
			}

			_, err = dbpool.Exec(r.Context(), `
			INSERT INTO collection_books (user_id, collection_id, book_id)
			VALUES ($1,$2,$3)
			ON CONFLICT DO NOTHING
		`, userID, body.CollectionID, bookID)
			if err != nil {
				http.Error(w, "DB insert error: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		writeJSON(w, map[string]any{"ok": true})
	})
	http.HandleFunc("/api/books/reviews/", booksReviewsHandler(dbpool, jwt))

	gb := httpapi.NewGoogleBooksHandler()

	http.HandleFunc("/api/google/search", gb.Search)
	http.HandleFunc("/api/books/google/", gb.GetByID)
	http.HandleFunc("/api/me/stats", statsHandler(dbpool, jwt))

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func nullIfZero(v int) any {
	if v == 0 {
		return nil
	}
	return v
}

func isValidStatus(s string) bool {
	switch s {
	case "planned", "reading", "finished", "dropped":
		return true
	default:
		return false
	}
}

func maturityToAge(m string) string {
	if m == "MATURE" {
		return "18+"
	}
	return ""
}

func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if i > start {
				parts = append(parts, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		parts = append(parts, s[start:])
	}
	return parts
}

func writeJSON(w http.ResponseWriter, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "JSON error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
func enableCORS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
}

func mustAuth(r *http.Request, jwt *auth.JWT) (int, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || h[:len(prefix)] != prefix {
		return 0, false
	}
	token := h[len(prefix):]
	claims, err := jwt.ParseToken(token)
	if err != nil {
		return 0, false
	}
	return int(claims.UserID), true
}
func statsHandler(dbpool *pgxpool.Pool, jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := mustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// 1) жанры прочитанных (finished)
		rows, err := dbpool.Query(r.Context(), `
      SELECT g.name AS genre, COUNT(*)::int AS cnt
      FROM user_books ub
      JOIN book_genres bg ON bg.book_id = ub.book_id
      JOIN genres g ON g.id = bg.genre_id
      WHERE ub.user_id = $1 AND ub.status = 'finished'
      GROUP BY g.name
      ORDER BY cnt DESC;
    `, userID)
		if err != nil {
			http.Error(w, "DB query error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		genres := make([]GenreStatDto, 0, 16)
		for rows.Next() {
			var dto GenreStatDto
			if err := rows.Scan(&dto.Genre, &dto.Cnt); err != nil {
				http.Error(w, "DB scan error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			genres = append(genres, dto)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "DB rows error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 2) добавления по месяцам (последние 6)
		rows2, err := dbpool.Query(r.Context(), `
      SELECT to_char(date_trunc('month', ub.created_at), 'YYYY-MM') AS month,
             COUNT(*)::int AS cnt
      FROM user_books ub
      WHERE ub.user_id = $1
        AND ub.created_at >= (date_trunc('month', now()) - interval '5 months')
      GROUP BY 1
      ORDER BY 1;
    `, userID)
		if err != nil {
			http.Error(w, "DB query error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows2.Close()

		months := make([]MonthStatDto, 0, 8)
		for rows2.Next() {
			var dto MonthStatDto
			if err := rows2.Scan(&dto.Month, &dto.Cnt); err != nil {
				http.Error(w, "DB scan error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			months = append(months, dto)
		}
		if err := rows2.Err(); err != nil {
			http.Error(w, "DB rows error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSONStatus(w, http.StatusOK, StatsResponse{
			Genres: genres,
			Months: months,
		})
	}
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "JSON error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

func booksReviewsHandler(dbpool *pgxpool.Pool, jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		googleId := strings.TrimPrefix(r.URL.Path, "/api/books/reviews/")
		googleId = strings.Trim(googleId, "/")
		if googleId == "" {
			http.NotFound(w, r)
			return
		}
		log.Println("reviews path:", r.URL.Path, "googleId:", googleId)

		var bookID int

		if err := dbpool.QueryRow(r.Context(),
			`SELECT id FROM books WHERE google_id=$1`, googleId,
		).Scan(&bookID); err != nil {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			rows, err := dbpool.Query(r.Context(), `
        SELECT r.id,
               COALESCE(NULLIF(u.name,''), u.email, 'User') AS user_name,
               r.created_at,
               r.rating,
               r.text
        FROM reviews r
        JOIN users u ON u.id = r.user_id
        WHERE r.book_id = $1
        ORDER BY r.created_at DESC;
      `, bookID)
			if err != nil {
				http.Error(w, "DB query error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			out := make([]ReviewDto, 0, 16)
			for rows.Next() {
				var dto ReviewDto
				var createdAt time.Time
				if err := rows.Scan(&dto.ID, &dto.UserName, &createdAt, &dto.Rating, &dto.Text); err != nil {
					http.Error(w, "DB scan error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				dto.CreatedAt = createdAt.Format("2006-01-02 15:04")
				out = append(out, dto)
			}
			if err := rows.Err(); err != nil {
				http.Error(w, "DB rows error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			writeJSONStatus(w, http.StatusOK, out)
			return

		case http.MethodPost:
			userID, ok := mustAuth(r, jwt)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			var body CreateReviewRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}

			body.Text = strings.TrimSpace(body.Text)
			if body.Rating < 1 || body.Rating > 5 {
				http.Error(w, "rating must be 1..5", http.StatusBadRequest)
				return
			}
			if body.Text == "" {
				http.Error(w, "text is required", http.StatusBadRequest)
				return
			}

			var reviewID int
			var createdAt time.Time
			err := dbpool.QueryRow(r.Context(), `
        INSERT INTO reviews (user_id, book_id, rating, text, created_at)
        VALUES ($1,$2,$3,$4, now())
        ON CONFLICT (user_id, book_id)
        DO UPDATE SET rating = EXCLUDED.rating,
                      text   = EXCLUDED.text,
                      created_at = now()
        RETURNING id, created_at;
      `, userID, bookID, body.Rating, body.Text).Scan(&reviewID, &createdAt)
			if err != nil {
				http.Error(w, "DB upsert error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			var userName string
			_ = dbpool.QueryRow(r.Context(), `
        SELECT COALESCE(NULLIF(name,''), email, 'User') FROM users WHERE id=$1
      `, userID).Scan(&userName)

			dto := ReviewDto{
				ID:        reviewID,
				UserName:  userName,
				CreatedAt: createdAt.Format("2006-01-02 15:04"),
				Rating:    body.Rating,
				Text:      body.Text,
			}

			writeJSONStatus(w, http.StatusCreated, dto)
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	}

}
