package handlers

import (
	"bookpulse/internal/db"
	"bookpulse/internal/models"
	"bookpulse/internal/service/auth"

	"encoding/json"
	"net/http"
)

type StatsResponse struct {
	Genres []models.GenreStatDto `json:"genres"`
	Months []models.MonthStatDto `json:"months"`
}

func StatsHandler(jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := auth.MustAuth(r, jwt)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		rows, err := db.DBpool.Query(r.Context(), `
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

		genres := make([]models.GenreStatDto, 0, 16)
		for rows.Next() {
			var dto models.GenreStatDto
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

		rows2, err := db.DBpool.Query(r.Context(), `
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

		months := make([]models.MonthStatDto, 0, 8)
		for rows2.Next() {
			var dto models.MonthStatDto
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