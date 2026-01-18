package models

type MyBookDTO struct {
	BookID      int      `json:"bookId"`
	GoogleID    string   `json:"googleId"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	CoverURL    string   `json:"coverUrl"`
	Status      string   `json:"status"`
	Collections []string `json:"collections"`
}