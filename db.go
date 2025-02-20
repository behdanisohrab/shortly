package main

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("sqlite3", "shortly.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	createTable()
}

func createTable() {
	query := `CREATE TABLE IF NOT EXISTS urls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT UNIQUE,
		url TEXT NOT NULL
	);`
	if _, err := db.Exec(query); err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

func SaveURL(code, url string) error {
	_, err := db.Exec("INSERT INTO urls (code, url) VALUES (?, ?)", code, url)
	return err
}

func GetURL(code string) (string, error) {
	var originalURL string
	row := db.QueryRow("SELECT url FROM urls WHERE code = ?", code)
	if err := row.Scan(&originalURL); err != nil {
		return "", err
	}
	return originalURL, nil
}

func URLExists(code string) (bool, error) {
	var exists bool
	row := db.QueryRow("SELECT EXISTS(SELECT 1 FROM urls WHERE code=?)", code)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func CloseDB() {
	db.Close()
}

