package main

import (
	"bookpulse/internal/db"
	"bookpulse/internal/google"
	"bookpulse/internal/handlers"
	"bookpulse/internal/middleware"
	"bookpulse/internal/repo"
	"bookpulse/internal/service/auth"
	"log"
	"net/http"
)

func main() {
	db.InitDB()
	googleBooks := google.NewGoogleBooksHandler()
	defer db.DBpool.Close()
	http.HandleFunc("/api/health", handlers.Health)

	jwtSecret := "dev_secret_change_me"
	jwt := auth.NewJWT(jwtSecret)
	userRepo := repo.NewUserRepoPGX(db.DBpool)
	authSvc := auth.NewServicePGX(userRepo, jwt)

	http.HandleFunc("/api/books/google", googleBooks.Search)

    http.HandleFunc("/api/books/google/", googleBooks.GetByID)

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

	http.HandleFunc("/api/books/reviews/", handlers.BooksReviewsHandler(jwt))

	log.Fatal(http.ListenAndServe(":8080", middleware.WithCORS(http.DefaultServeMux)))
}

