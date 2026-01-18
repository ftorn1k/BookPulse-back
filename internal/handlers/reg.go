package handlers

import (
	"bookpulse/internal/service/auth"
	"bookpulse/internal/utils"
	"encoding/json"
	"net/http"
)

func Register(authSvc *auth.ServicePGX) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		utils.EnableCORS(w, r)
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

		resp, err := authSvc.Register(
			r.Context(),
			body.Email,
			body.Password,
			body.Name,
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		utils.WriteJSON(w, resp)
	}
}
