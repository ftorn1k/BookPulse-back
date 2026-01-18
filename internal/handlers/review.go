package handlers

import (
	"bookpulse/internal/db"
	"bookpulse/internal/models"
	"bookpulse/internal/service/auth"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

func BooksReviewsHandler(jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		if err := db.DBpool.QueryRow(r.Context(),
			`SELECT id FROM books WHERE google_id=$1`, googleId,
		).Scan(&bookID); err != nil {
			http.Error(w, "book not found", http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			rows, err := db.DBpool.Query(r.Context(), `
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

			out := make([]models.ReviewDto, 0, 16)
			for rows.Next() {
				var dto models.ReviewDto
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
			userID, ok := auth.MustAuth(r, jwt)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			
			type CreateReviewRequest struct {
				Rating int    `json:"rating"`
				Text   string `json:"text"`
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
			err := db.DBpool.QueryRow(r.Context(), `
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
			_ = db.DBpool.QueryRow(r.Context(), `
        SELECT COALESCE(NULLIF(name,''), email, 'User') FROM users WHERE id=$1
      `, userID).Scan(&userName)

			dto := models.ReviewDto{
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