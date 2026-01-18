package handlers

import (
	"bookpulse/internal/db"
	"bookpulse/internal/service/auth"
	"bookpulse/internal/utils"
	"encoding/json"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func CurrentUser(authSvc *auth.ServicePGX, jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.EnableCORS(w, r)
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

		me, err := authSvc.Me(r.Context(), userID)
		if err != nil {
			http.Error(w, "user not found", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		utils.WriteJSON(w, me)
	}
}

type UpdateProfileRequest struct {
	Name string `json:"name"`
}

func UpdateName(jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.EnableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := auth.MustAuth(r, jwt)
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

		_, err := db.DBpool.Exec(r.Context(),
			`UPDATE users SET name=$2 WHERE id=$1`,
			userID, name,
		)
		if err != nil {
			http.Error(w, "DB update error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		utils.WriteJSON(w, map[string]any{"ok": true, "name": name})
	}
}

type UpdatePasswordRequest struct {
	Password string `json:"password"`
}

func UpdatePassword(jwt *auth.JWT) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.EnableCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodPatch {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, ok := auth.MustAuth(r, jwt)
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

		_, err = db.DBpool.Exec(r.Context(),
			`UPDATE users SET password_hash=$2 WHERE id=$1`,
			userID, string(hash),
		)
		if err != nil {
			http.Error(w, "DB update error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		utils.WriteJSON(w, map[string]any{"ok": true})
	}
}
