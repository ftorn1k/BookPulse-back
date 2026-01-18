package db

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DBpool *pgxpool.Pool

func InitDB() {
	dsn := "host=127.0.0.1 port=5433 user=bookpulse password=bookpulse dbname=bookpulse sslmode=disable"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	DBpool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal("Ошибка подключения к БД:", err)
	}
	defer DBpool.Close()

	if err := DBpool.Ping(ctx); err != nil {
		log.Fatal("БД не отвечает:", err)
	}
	_, err = DBpool.Exec(ctx, `
	CREATE TABLE IF NOT EXISTS users (
	id SERIAL PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now()
	);
	`)
	if err != nil {
		log.Fatal("Не удалось создать таблицу users:", err)
	}

	log.Println("Успешное подключение к PostgreSQL")
}