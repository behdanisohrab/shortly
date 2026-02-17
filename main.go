package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	codeLength = 6
	charset    = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

var domain string
var port string

func init() {
	rand.Seed(time.Now().UnixNano())

	if err := godotenv.Load(); err != nil {
		fmt.Println("Warning: .env file not found, using defaults")
	}

	domain = os.Getenv("DOMAIN")
	if domain == "" {
		domain = "localhost"
	}

	port = os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	InitDB()
	initAuth()
	ensureAdmin()
	initRateLimiter(30, time.Minute)

	go cleanupExpiredURLs()
}

func cleanupExpiredURLs() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		DeleteExpiredURLs()
		DeleteMaxedOutURLs()
	}
}

func generateRandomCode() string {
	b := make([]byte, codeLength)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func buildShortURL(code string) string {
	if domain == "localhost" {
		return fmt.Sprintf("http://%s:%s/%s", domain, port, code)
	}
	return fmt.Sprintf("https://%s/%s", domain, code)
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}

func isValidURL(rawURL string) bool {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

var reservedCodes = map[string]bool{
	"create": true, "api": true, "static": true, "admin": true,
	"login": true, "register": true, "logout": true, "dashboard": true,
	"auth": true, "user": true, "urls": true, "stats": true,
	"qr": true, "blacklist": true, "health": true,
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Username == "" || req.Email == "" || req.Password == "" {
		jsonError(w, http.StatusBadRequest, "Username, email, and password are required")
		return
	}

	if len(req.Password) < 6 {
		jsonError(w, http.StatusBadRequest, "Password must be at least 6 characters")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	user, err := CreateUser(req.Username, req.Email, hash)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, http.StatusConflict, "Username or email already exists")
			return
		}
		jsonError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	token, err := GenerateJWT(user)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   259200,
		SameSite: http.SameSiteLaxMode,
	})

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := GetUserByUsername(req.Username)
	if err != nil {
		jsonError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	if !CheckPassword(req.Password, user.PasswordHash) {
		jsonError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	token, err := GenerateJWT(user)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   259200,
		SameSite: http.SameSiteLaxMode,
	})

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	jsonResponse(w, http.StatusOK, map[string]string{"message": "Logged out"})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r)
	jsonResponse(w, http.StatusOK, user)
}

func createShortURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var request struct {
		URL          string `json:"url"`
		CustomCode   string `json:"custom_code"`
		RedirectType int    `json:"redirect_type"`
		ExpiresIn    int    `json:"expires_in"`
		MaxUses      int    `json:"max_uses"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if request.URL == "" {
		jsonError(w, http.StatusBadRequest, "URL is required")
		return
	}

	if !isValidURL(request.URL) {
		jsonError(w, http.StatusBadRequest, "Invalid URL. Must start with http:// or https://")
		return
	}

	urlDomain := extractDomain(request.URL)
	if blacklisted, _ := IsBlacklisted(urlDomain); blacklisted {
		jsonError(w, http.StatusForbidden, "This URL domain is blacklisted")
		return
	}

	if request.RedirectType != 301 && request.RedirectType != 302 {
		request.RedirectType = 302
	}

	var code string
	isCustom := false

	if request.CustomCode != "" {
		code = request.CustomCode
		isCustom = true

		if len(code) < 3 || len(code) > 20 {
			jsonError(w, http.StatusBadRequest, "Custom code must be 3-20 characters")
			return
		}

		if reservedCodes[code] {
			jsonError(w, http.StatusConflict, "This code is reserved")
			return
		}

		exists, _ := URLExists(code)
		if exists {
			jsonError(w, http.StatusConflict, "Custom code already in use")
			return
		}
	} else {
		for {
			code = generateRandomCode()
			exists, _ := URLExists(code)
			if !exists {
				break
			}
		}
	}

	var expiresAt *time.Time
	if request.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(request.ExpiresIn) * time.Hour)
		expiresAt = &t
	}

	user := getUserFromContext(r)
	var userID *int
	if user != nil {
		userID = &user.ID
	}

	if err := SaveURL(code, request.URL, userID, isCustom, request.RedirectType, expiresAt, request.MaxUses); err != nil {
		jsonError(w, http.StatusInternalServerError, "Error saving URL")
		return
	}

	shortURL := buildShortURL(code)

	jsonResponse(w, http.StatusCreated, map[string]interface{}{
		"short_url":     shortURL,
		"code":          code,
		"redirect_type": request.RedirectType,
		"expires_at":    expiresAt,
		"max_uses":      request.MaxUses,
	})
}

func handleMyURLs(w http.ResponseWriter, r *http.Request) {
	user := getUserFromContext(r)
	urls, err := GetURLsByUserID(user.ID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to fetch URLs")
		return
	}
	if urls == nil {
		urls = []URLWithStats{}
	}
	for i := range urls {
		urls[i].Code = buildShortURL(urls[i].Code)
	}
	jsonResponse(w, http.StatusOK, urls)
}

func handleDeleteURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid URL ID")
		return
	}

	user := getUserFromContext(r)
	if err := DeleteURL(id, user.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to delete URL")
		return
	}

	jsonResponse(w, http.StatusOK, map[string]string{"message": "URL deleted"})
}

func handleURLStats(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		jsonError(w, http.StatusBadRequest, "Code is required")
		return
	}

	urlRecord, err := GetURL(code)
	if err != nil {
		jsonError(w, http.StatusNotFound, "URL not found")
		return
	}

	clicks, err := GetClicksForURL(urlRecord.ID)
	if err != nil {
		clicks = []Click{}
	}

	totalClicks, _ := GetClickCountForURL(urlRecord.ID)

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"url":          urlRecord,
		"total_clicks": totalClicks,
		"clicks":       clicks,
		"short_url":    buildShortURL(urlRecord.Code),
	})
}

func handleQRCode(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		jsonError(w, http.StatusBadRequest, "Code is required")
		return
	}

	shortURL := buildShortURL(code)

	sizeStr := r.URL.Query().Get("size")
	size := 256
	if sizeStr != "" {
		if s, err := strconv.Atoi(sizeStr); err == nil && s >= 64 && s <= 1024 {
			size = s
		}
	}

	png, err := qrcode.Encode(shortURL, qrcode.Medium, size)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to generate QR code")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(png)
}

func handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := GetStats()
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to fetch stats")
		return
	}
	jsonResponse(w, http.StatusOK, stats)
}

func handleAdminURLs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	urls, total, err := GetAllURLs(limit, offset)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to fetch URLs")
		return
	}
	if urls == nil {
		urls = []URLWithStats{}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"urls":  urls,
		"total": total,
	})
}

func handleAdminUsers(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 50
	}

	users, total, err := GetAllUsers(limit, offset)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to fetch users")
		return
	}
	if users == nil {
		users = []User{}
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"users": users,
		"total": total,
	})
}

func handleAdminDeleteURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid URL ID")
		return
	}

	if err := DeleteURLAdmin(id); err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to delete URL")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "URL deleted"})
}

func handleAdminBlacklist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		list, err := GetBlacklist()
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "Failed to fetch blacklist")
			return
		}
		if list == nil {
			list = []map[string]string{}
		}
		jsonResponse(w, http.StatusOK, list)

	case http.MethodPost:
		var req struct {
			Domain string `json:"domain"`
			Reason string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, http.StatusBadRequest, "Invalid request")
			return
		}
		if err := AddToBlacklist(req.Domain, req.Reason); err != nil {
			jsonError(w, http.StatusInternalServerError, "Failed to add to blacklist")
			return
		}
		jsonResponse(w, http.StatusCreated, map[string]string{"message": "Domain blacklisted"})

	case http.MethodDelete:
		domain := r.URL.Query().Get("domain")
		if err := RemoveFromBlacklist(domain); err != nil {
			jsonError(w, http.StatusInternalServerError, "Failed to remove from blacklist")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"message": "Domain removed from blacklist"})

	default:
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func handleAdminSetAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		UserID  int  `json:"user_id"`
		IsAdmin bool `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if err := SetUserAdmin(req.UserID, req.IsAdmin); err != nil {
		jsonError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "User updated"})
}

func redirectShortURL(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Path[1:]

	if code == "" {
		http.ServeFile(w, r, "static/index.html")
		return
	}

	urlRecord, err := GetURL(code)
	if err != nil {
		http.ServeFile(w, r, "static/404.html")
		return
	}

	if urlRecord.ExpiresAt != nil && urlRecord.ExpiresAt.Before(time.Now()) {
		http.ServeFile(w, r, "static/404.html")
		return
	}

	if urlRecord.MaxUses > 0 && urlRecord.UseCount >= urlRecord.MaxUses {
		http.ServeFile(w, r, "static/404.html")
		return
	}

	IncrementUseCount(urlRecord.ID)
	go RecordClick(urlRecord.ID, getClientIP(r), r.UserAgent(), r.Referer(), "")

	http.Redirect(w, r, urlRecord.OriginalURL, urlRecord.RedirectType)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func main() {
	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("/api/auth/register", rateLimitMiddleware(handleRegister))
	mux.HandleFunc("/api/auth/login", rateLimitMiddleware(handleLogin))
	mux.HandleFunc("/api/auth/logout", handleLogout)
	mux.HandleFunc("/api/auth/me", authMiddleware(handleMe))

	mux.HandleFunc("/api/urls/create", rateLimitMiddleware(optionalAuthMiddleware(createShortURL)))
	mux.HandleFunc("/api/urls/my", authMiddleware(handleMyURLs))
	mux.HandleFunc("/api/urls/delete", authMiddleware(handleDeleteURL))
	mux.HandleFunc("/api/urls/stats", handleURLStats)

	mux.HandleFunc("/api/qr", handleQRCode)

	mux.HandleFunc("/api/admin/stats", adminMiddleware(handleAdminStats))
	mux.HandleFunc("/api/admin/urls", adminMiddleware(handleAdminURLs))
	mux.HandleFunc("/api/admin/users", adminMiddleware(handleAdminUsers))
	mux.HandleFunc("/api/admin/urls/delete", adminMiddleware(handleAdminDeleteURL))
	mux.HandleFunc("/api/admin/blacklist", adminMiddleware(handleAdminBlacklist))
	mux.HandleFunc("/api/admin/set-admin", adminMiddleware(handleAdminSetAdmin))

	mux.HandleFunc("/api/health", handleHealth)

	mux.HandleFunc("/", redirectShortURL)

	fmt.Printf("Shortly server started on http://%s:%s\n", domain, port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), corsMiddleware(mux)); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
