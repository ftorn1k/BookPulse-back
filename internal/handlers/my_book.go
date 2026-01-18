package handlers

import (
	"bookpulse/internal/db"
	"bookpulse/internal/models"
	"bookpulse/internal/service/auth"
	"bookpulse/internal/utils"
	"encoding/json"
	"net/http"
	"strings"
)

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


func GetAndAddMyBook(jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		userID, ok := auth.MustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {

		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			rows, err := db.DBpool.Query(r.Context(), `
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

			result := make([]models.MyBookDTO, 0, 16)
			for rows.Next() {
				var dto models.MyBookDTO
				var collectionsCSV string

				if err := rows.Scan(&dto.BookID, &dto.GoogleID, &dto.Title, &dto.Author, &dto.CoverURL, &dto.Status, &collectionsCSV); err != nil {
					http.Error(w, "DB scan error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				dto.Collections = utils.SplitCSV(collectionsCSV)
				result = append(result, dto)
			}
			if err := rows.Err(); err != nil {
				http.Error(w, "DB rows error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			utils.WriteJSON(w, result)
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
			err := db.DBpool.QueryRow(r.Context(), `
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
				utils.NullIfZero(body.PublishedYear),
				utils.NullIfZero(body.PageCount),
				utils.MaturityToAge(body.Maturity),
			).Scan(&bookID)
			if err != nil {
				http.Error(w, "DB insert book error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			for _, raw := range body.Categories {
				parts := strings.Split(raw, "/")

				for i := 0; i < len(parts) && i < 2; i++ {
					g := strings.TrimSpace(parts[i])
					if g == "" || g == "General" {
						continue
					}

					var genreID int
					err := db.DBpool.QueryRow(r.Context(), `
      INSERT INTO genres (name)
      VALUES ($1)
      ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
      RETURNING id;
    `, g).Scan(&genreID)
					if err != nil {
						http.Error(w, "DB insert genre error: "+err.Error(), http.StatusInternalServerError)
						return
					}

					_, err = db.DBpool.Exec(r.Context(), `
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

			_, err = db.DBpool.Exec(r.Context(), `
  INSERT INTO user_books (user_id, book_id, status)
  VALUES ($1,$2,$3)
  ON CONFLICT (user_id, book_id) DO UPDATE SET status = EXCLUDED.status;
`, userID, bookID, body.Status)

			_, err = db.DBpool.Exec(r.Context(), `
			INSERT INTO user_books (user_id, book_id, status)
			VALUES ($1,$2,$3)
			ON CONFLICT (user_id, book_id) DO UPDATE SET status = EXCLUDED.status;
		`, userID, bookID, body.Status)
			if err != nil {
				http.Error(w, "DB insert user_books error: "+err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			utils.WriteJSON(w, map[string]any{
				"ok":       true,
				"bookId":   bookID,
				"googleId": body.GoogleID,
			})
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}
}


type UpdateStatusRequest struct {
	GoogleID string `json:"googleId"`
	BookID   int    `json:"bookId"`
	Status   string `json:"status"`
}


func SetStatus(jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		userID, ok := auth.MustAuth(r, jwt)
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
		if !utils.IsValidStatus(body.Status) {
			http.Error(w, "invalid status", http.StatusBadRequest)
			return
		}

		bookID := body.BookID
		if bookID == 0 {
			if body.GoogleID == "" {
				http.Error(w, "googleId or bookId required", http.StatusBadRequest)
				return
			}
			if err := db.DBpool.QueryRow(r.Context(), `SELECT id FROM books WHERE google_id=$1`, body.GoogleID).Scan(&bookID); err != nil {
				http.Error(w, "book not found", http.StatusBadRequest)
				return
			}
		}

		cmd, err := db.DBpool.Exec(r.Context(), `
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
		utils.WriteJSON(w, map[string]any{"ok": true})
	}
}