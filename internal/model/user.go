package model

import "time"

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"not null" json:"-"` 
	Name      string    `gorm:"" json:"name"`      
	CreatedAt time.Time `json:"createdAt"`
}
