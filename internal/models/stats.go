package models


type GenreStatDto struct {
	Genre string `json:"genre"`
	Cnt   int    `json:"cnt"`
}

type MonthStatDto struct {
	Month string `json:"month"`
	Cnt   int    `json:"cnt"`
}