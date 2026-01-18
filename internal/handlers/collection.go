package handlers

import (
	"bookpulse/internal/db"
	"bookpulse/internal/models"
	"bookpulse/internal/service/auth"
	"bookpulse/internal/utils"
	"encoding/json"
	"net/http"
)

type CreateCollectionRequest struct {
	Name    string `json:"name"`
	BookIDs []int  `json:"bookIds"`
}

func GetAndAddCollection(jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.EnableCORS(w, r)

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
			rows, err := db.DBpool.Query(r.Context(), `
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

			out := make([]models.MyCollectionDTO, 0, 16)
			for rows.Next() {
				var dto models.MyCollectionDTO
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
			utils.WriteJSON(w, out)
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
			err := db.DBpool.QueryRow(r.Context(), `
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
				if err := db.DBpool.QueryRow(r.Context(), `
			SELECT EXISTS(SELECT 1 FROM user_books WHERE user_id=$1 AND book_id=$2)
		`, userID, bookID).Scan(&inLib); err != nil {
					http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
					return
				}
				if !inLib {
					http.Error(w, "book not in user's library", http.StatusBadRequest)
					return
				}

				_, err := db.DBpool.Exec(r.Context(), `
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
			utils.WriteJSON(w, map[string]any{
				"ok":           true,
				"collectionId": collectionID,
			})
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}
}

type AddBooksToCollectionRequest struct {
	CollectionID int      `json:"collectionId"`
	GoogleIDs    []string `json:"googleIds"`
}

func AddBookToCollection(jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.EnableCORS(w, r)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		userID, ok := auth.MustAuth(r, jwt)
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
		if err := db.DBpool.QueryRow(r.Context(), `
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
			err := db.DBpool.QueryRow(r.Context(), `SELECT id FROM books WHERE google_id=$1`, gid).Scan(&bookID)
			if err != nil {
				http.Error(w, "book not found: "+gid, http.StatusBadRequest)
				return
			}

			var inLib bool
			if err := db.DBpool.QueryRow(r.Context(), `
			SELECT EXISTS(SELECT 1 FROM user_books WHERE user_id=$1 AND book_id=$2)
		`, userID, bookID).Scan(&inLib); err != nil {
				http.Error(w, "DB error: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if !inLib {
				http.Error(w, "book not in user's library", http.StatusBadRequest)
				return
			}

			_, err = db.DBpool.Exec(r.Context(), `
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
		utils.WriteJSON(w, map[string]any{"ok": true})
	}
}
