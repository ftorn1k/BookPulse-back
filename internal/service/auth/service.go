package auth

import (
	"context"
	"errors"
	"log"

	"golang.org/x/crypto/bcrypt"

	"bookpulse/internal/repo"
)

type ServicePGX struct {
	users *repo.UserRepoPGX
	jwt   *JWT
}

func NewServicePGX(users *repo.UserRepoPGX, jwt *JWT) *ServicePGX {
	return &ServicePGX{users: users, jwt: jwt}
}

type UserDTO struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type AuthResponse struct {
	Token string  `json:"token"`
	User  UserDTO `json:"user"`
}

func (s *ServicePGX) Register(ctx context.Context, email, password, name string) (*AuthResponse, error) {
	existing, _ := s.users.FindByEmail(ctx, email)
	if existing != nil {
		return nil, errors.New("email already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u, err := s.users.Create(ctx, email, name, string(hash))
	if err != nil {
		return nil, err
	}

	token, err := s.jwt.GenerateToken(uint(u.ID))
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		Token: token,
		User:  UserDTO{ID: u.ID, Email: u.Email, Name: u.Name},
	}, nil
}

func (s *ServicePGX) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
 log.Printf("LOGIN email=%q pass_len=%d", email, len(password))
 u, err := s.users.FindByEmail(ctx, email)
 if err != nil {
	log.Printf("LOGIN FindByEmail error: %v", err)
  return nil, err 
 }
 if u == nil {
	log.Printf("LOGIN user not found")
  return nil, repo.ErrInvalidCredentials
 }

 if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
	log.Printf("LOGIN bcrypt compare error: %v", err)
  return nil, repo.ErrInvalidCredentials
 }

 token, err := s.jwt.GenerateToken(uint(u.ID))
 if err != nil {
  return nil, err
 }

 return &AuthResponse{
  Token: token,
  User:  UserDTO{ID: u.ID, Email: u.Email, Name: u.Name},
 }, nil
}


func (s *ServicePGX) Me(ctx context.Context, userID int) (*UserDTO, error) {
	u, _ := s.users.FindByID(ctx, userID)
	if u == nil {
		return nil, errors.New("user not found")
	}
	dto := &UserDTO{ID: u.ID, Email: u.Email, Name: u.Name}
	return dto, nil
}
