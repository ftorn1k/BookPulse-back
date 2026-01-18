package models

type ReviewDto struct {
	ID        int    `json:"id"`
	UserName  string `json:"userName"`
	CreatedAt string `json:"createdAt"`
	Rating    int    `json:"rating"`
	Text      string `json:"text"`
}