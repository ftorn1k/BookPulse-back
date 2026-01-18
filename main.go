package main

import (
	"log"
	"net/http"

	"bookpulse/internal/db"
	"bookpulse/internal/handlers"
	"bookpulse/internal/repo"
	"bookpulse/internal/service/auth"
)

func main() {
	db.InitDB()

	http.HandleFunc("/api/health", handlers.Health)

	jwtSecret := "dev_secret_change_me"
	jwt := auth.NewJWT(jwtSecret)
	userRepo := repo.NewUserRepoPGX(db.DBpool)
	authSvc := auth.NewServicePGX(userRepo, jwt)

	http.HandleFunc("/api/auth/register", handlers.Register(authSvc))

	http.HandleFunc("/api/auth/login", handlers.Authorization(authSvc))
	
	http.HandleFunc("/api/auth/me", handlers.CurrentUser(authSvc, jwt))

	http.HandleFunc("/api/me/profile", handlers.UpdateName(jwt))

	http.HandleFunc("/api/me/password", handlers.UpdatePassword(jwt))

	http.HandleFunc("/api/me/books", handlers.GetAndAddMyBook(jwt))

	http.HandleFunc("/api/me/books/status", handlers.SetStatus(jwt))

	http.HandleFunc("/api/me/collections", handlers.GetAndAddCollection(jwt))

	http.HandleFunc("/api/me/collections/add-books", handlers.AddBookToCollection(jwt)) 

	http.HandleFunc("/api/me/stats", handlers.StatsHandler(jwt)) 

	log.Fatal(http.ListenAndServe(":8080", nil))
}

