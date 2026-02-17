package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func InitDB() {
	var err error
	db, err = sql.Open("sqlite3", "shortly.db?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	createTables()
}

func createTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			is_admin BOOLEAN DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS urls (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			code TEXT UNIQUE NOT NULL,
			url TEXT NOT NULL,
			user_id INTEGER,
			custom_code BOOLEAN DEFAULT FALSE,
			redirect_type INTEGER DEFAULT 302,
			expires_at DATETIME,
			max_uses INTEGER DEFAULT 0,
			use_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		);`,
		`CREATE TABLE IF NOT EXISTS clicks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url_id INTEGER NOT NULL,
			ip_address TEXT,
			user_agent TEXT,
			referer TEXT,
			country TEXT,
			clicked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (url_id) REFERENCES urls(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS blacklist (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			domain TEXT UNIQUE NOT NULL,
			reason TEXT,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_urls_code ON urls(code);`,
		`CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls(user_id);`,
		`CREATE INDEX IF NOT EXISTS idx_clicks_url_id ON clicks(url_id);`,
		`CREATE INDEX IF NOT EXISTS idx_clicks_clicked_at ON clicks(clicked_at);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			log.Fatalf("Failed to execute query: %v\nQuery: %s", err, q)
		}
	}
}

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	IsAdmin      bool      `json:"is_admin"`
	CreatedAt    time.Time `json:"created_at"`
}

type URL struct {
	ID           int        `json:"id"`
	Code         string     `json:"code"`
	OriginalURL  string     `json:"url"`
	UserID       *int       `json:"user_id,omitempty"`
	CustomCode   bool       `json:"custom_code"`
	RedirectType int        `json:"redirect_type"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	MaxUses      int        `json:"max_uses"`
	UseCount     int        `json:"use_count"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Click struct {
	ID        int       `json:"id"`
	URLID     int       `json:"url_id"`
	IPAddress string    `json:"ip_address"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
	Country   string    `json:"country"`
	ClickedAt time.Time `json:"clicked_at"`
}

type URLWithStats struct {
	URL
	TotalClicks int `json:"total_clicks"`
}

func CreateUser(username, email, passwordHash string) (*User, error) {
	result, err := db.Exec(
		"INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)",
		username, email, passwordHash,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return &User{ID: int(id), Username: username, Email: email}, nil
}

func GetUserByUsername(username string) (*User, error) {
	var u User
	err := db.QueryRow(
		"SELECT id, username, email, password_hash, is_admin, created_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func GetUserByID(id int) (*User, error) {
	var u User
	err := db.QueryRow(
		"SELECT id, username, email, password_hash, is_admin, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func SaveURL(code, url string, userID *int, customCode bool, redirectType int, expiresAt *time.Time, maxUses int) error {
	_, err := db.Exec(
		"INSERT INTO urls (code, url, user_id, custom_code, redirect_type, expires_at, max_uses) VALUES (?, ?, ?, ?, ?, ?, ?)",
		code, url, userID, customCode, redirectType, expiresAt, maxUses,
	)
	return err
}

func GetURL(code string) (*URL, error) {
	var u URL
	var userID sql.NullInt64
	var expiresAt sql.NullTime
	err := db.QueryRow(
		"SELECT id, code, url, user_id, custom_code, redirect_type, expires_at, max_uses, use_count, created_at FROM urls WHERE code = ?",
		code,
	).Scan(&u.ID, &u.Code, &u.OriginalURL, &userID, &u.CustomCode, &u.RedirectType, &expiresAt, &u.MaxUses, &u.UseCount, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	if userID.Valid {
		uid := int(userID.Int64)
		u.UserID = &uid
	}
	if expiresAt.Valid {
		u.ExpiresAt = &expiresAt.Time
	}
	return &u, nil
}

func IncrementUseCount(urlID int) error {
	_, err := db.Exec("UPDATE urls SET use_count = use_count + 1 WHERE id = ?", urlID)
	return err
}

func URLExists(code string) (bool, error) {
	var exists bool
	row := db.QueryRow("SELECT EXISTS(SELECT 1 FROM urls WHERE code=?)", code)
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func RecordClick(urlID int, ipAddress, userAgent, referer, country string) error {
	_, err := db.Exec(
		"INSERT INTO clicks (url_id, ip_address, user_agent, referer, country) VALUES (?, ?, ?, ?, ?)",
		urlID, ipAddress, userAgent, referer, country,
	)
	return err
}

func GetClicksForURL(urlID int) ([]Click, error) {
	rows, err := db.Query(
		"SELECT id, url_id, ip_address, user_agent, referer, country, clicked_at FROM clicks WHERE url_id = ? ORDER BY clicked_at DESC",
		urlID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clicks []Click
	for rows.Next() {
		var c Click
		if err := rows.Scan(&c.ID, &c.URLID, &c.IPAddress, &c.UserAgent, &c.Referer, &c.Country, &c.ClickedAt); err != nil {
			return nil, err
		}
		clicks = append(clicks, c)
	}
	return clicks, nil
}

func GetClickCountForURL(urlID int) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM clicks WHERE url_id = ?", urlID).Scan(&count)
	return count, err
}

func GetClicksByDay(urlID int, days int) ([]map[string]interface{}, error) {
	rows, err := db.Query(
		`SELECT DATE(clicked_at) as day, COUNT(*) as count 
		FROM clicks WHERE url_id = ? AND clicked_at >= datetime('now', ?)
		GROUP BY DATE(clicked_at) ORDER BY day`,
		urlID, "-"+string(rune(days+'0'))+" days",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var day string
		var count int
		if err := rows.Scan(&day, &count); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{"day": day, "count": count})
	}
	return result, nil
}

func GetURLsByUserID(userID int) ([]URLWithStats, error) {
	rows, err := db.Query(
		`SELECT u.id, u.code, u.url, u.user_id, u.custom_code, u.redirect_type, u.expires_at, u.max_uses, u.use_count, u.created_at,
		COALESCE((SELECT COUNT(*) FROM clicks c WHERE c.url_id = u.id), 0) as total_clicks
		FROM urls u WHERE u.user_id = ? ORDER BY u.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var urls []URLWithStats
	for rows.Next() {
		var u URLWithStats
		var userIDVal sql.NullInt64
		var expiresAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.Code, &u.OriginalURL, &userIDVal, &u.CustomCode, &u.RedirectType, &expiresAt, &u.MaxUses, &u.UseCount, &u.CreatedAt, &u.TotalClicks); err != nil {
			return nil, err
		}
		if userIDVal.Valid {
			uid := int(userIDVal.Int64)
			u.UserID = &uid
		}
		if expiresAt.Valid {
			u.ExpiresAt = &expiresAt.Time
		}
		urls = append(urls, u)
	}
	return urls, nil
}

func DeleteURL(urlID int, userID int) error {
	_, err := db.Exec("DELETE FROM urls WHERE id = ? AND user_id = ?", urlID, userID)
	return err
}

func DeleteURLAdmin(urlID int) error {
	_, err := db.Exec("DELETE FROM urls WHERE id = ?", urlID)
	return err
}

func GetAllURLs(limit, offset int) ([]URLWithStats, int, error) {
	var total int
	db.QueryRow("SELECT COUNT(*) FROM urls").Scan(&total)

	rows, err := db.Query(
		`SELECT u.id, u.code, u.url, u.user_id, u.custom_code, u.redirect_type, u.expires_at, u.max_uses, u.use_count, u.created_at,
		COALESCE((SELECT COUNT(*) FROM clicks c WHERE c.url_id = u.id), 0) as total_clicks
		FROM urls u ORDER BY u.created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var urls []URLWithStats
	for rows.Next() {
		var u URLWithStats
		var userIDVal sql.NullInt64
		var expiresAt sql.NullTime
		if err := rows.Scan(&u.ID, &u.Code, &u.OriginalURL, &userIDVal, &u.CustomCode, &u.RedirectType, &expiresAt, &u.MaxUses, &u.UseCount, &u.CreatedAt, &u.TotalClicks); err != nil {
			return nil, 0, err
		}
		if userIDVal.Valid {
			uid := int(userIDVal.Int64)
			u.UserID = &uid
		}
		if expiresAt.Valid {
			u.ExpiresAt = &expiresAt.Time
		}
		urls = append(urls, u)
	}
	return urls, total, nil
}

func GetAllUsers(limit, offset int) ([]User, int, error) {
	var total int
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&total)

	rows, err := db.Query(
		"SELECT id, username, email, password_hash, is_admin, created_at FROM users ORDER BY created_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func IsBlacklisted(domain string) (bool, error) {
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM blacklist WHERE domain = ?)", domain).Scan(&exists)
	return exists, err
}

func AddToBlacklist(domain, reason string) error {
	_, err := db.Exec("INSERT OR IGNORE INTO blacklist (domain, reason) VALUES (?, ?)", domain, reason)
	return err
}

func RemoveFromBlacklist(domain string) error {
	_, err := db.Exec("DELETE FROM blacklist WHERE domain = ?", domain)
	return err
}

func GetBlacklist() ([]map[string]string, error) {
	rows, err := db.Query("SELECT domain, reason, added_at FROM blacklist ORDER BY added_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []map[string]string
	for rows.Next() {
		var domain, reason, addedAt string
		if err := rows.Scan(&domain, &reason, &addedAt); err != nil {
			return nil, err
		}
		list = append(list, map[string]string{"domain": domain, "reason": reason, "added_at": addedAt})
	}
	return list, nil
}

func DeleteExpiredURLs() (int64, error) {
	result, err := db.Exec("DELETE FROM urls WHERE expires_at IS NOT NULL AND expires_at < datetime('now')")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func DeleteMaxedOutURLs() (int64, error) {
	result, err := db.Exec("DELETE FROM urls WHERE max_uses > 0 AND use_count >= max_uses")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	var totalURLs, totalClicks, totalUsers int
	db.QueryRow("SELECT COUNT(*) FROM urls").Scan(&totalURLs)
	db.QueryRow("SELECT COUNT(*) FROM clicks").Scan(&totalClicks)
	db.QueryRow("SELECT COUNT(*) FROM users").Scan(&totalUsers)

	stats["total_urls"] = totalURLs
	stats["total_clicks"] = totalClicks
	stats["total_users"] = totalUsers

	rows, err := db.Query(
		`SELECT DATE(clicked_at) as day, COUNT(*) as count 
		FROM clicks WHERE clicked_at >= datetime('now', '-7 days')
		GROUP BY DATE(clicked_at) ORDER BY day`,
	)
	if err == nil {
		defer rows.Close()
		var dailyClicks []map[string]interface{}
		for rows.Next() {
			var day string
			var count int
			rows.Scan(&day, &count)
			dailyClicks = append(dailyClicks, map[string]interface{}{"day": day, "count": count})
		}
		stats["daily_clicks"] = dailyClicks
	}

	return stats, nil
}

func SetUserAdmin(userID int, isAdmin bool) error {
	_, err := db.Exec("UPDATE users SET is_admin = ? WHERE id = ?", isAdmin, userID)
	return err
}

func CloseDB() {
	db.Close()
}
