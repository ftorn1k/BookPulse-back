package repo

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           int    `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
}

type UserRepoPGX struct {
	db *pgxpool.Pool
}

func NewUserRepoPGX(db *pgxpool.Pool) *UserRepoPGX {
	return &UserRepoPGX{db: db}
}

func (r *UserRepoPGX) Create(ctx context.Context, email, name, passwordHash string) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (email, name, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, email, name, password_hash;
	`, email, name, passwordHash).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepoPGX) FindByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, name, password_hash
		FROM users
		WHERE email = $1;
	`, email).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash)

	if err != nil {
		return nil, nil
	}
	return &u, nil
}

func (r *UserRepoPGX) FindByID(ctx context.Context, id int) (*User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		SELECT id, email, name, password_hash
		FROM users
		WHERE id = $1;
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash)

	if err != nil {
		return nil, nil
	}
	return &u, nil
}

var ErrInvalidCredentials = errors.New("invalid credentials")
