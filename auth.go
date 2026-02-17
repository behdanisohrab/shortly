package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type contextKey string

const userContextKey contextKey = "user"

var jwtSecret []byte

func initAuth() {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "shortly-default-secret-change-me"
	}
	jwtSecret = []byte(secret)
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateJWT(user *User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"is_admin": user.IsAdmin,
		"exp":      time.Now().Add(72 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func ValidateJWT(tokenString string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ""
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if tokenStr == "" {
			cookie, err := r.Cookie("token")
			if err == nil {
				tokenStr = cookie.Value
			}
		}

		if tokenStr == "" {
			http.Error(w, `{"error":"Authentication required"}`, http.StatusUnauthorized)
			return
		}

		claims, err := ValidateJWT(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"Invalid token"}`, http.StatusUnauthorized)
			return
		}

		userID := int(claims["user_id"].(float64))
		user, err := GetUserByID(userID)
		if err != nil {
			http.Error(w, `{"error":"User not found"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func optionalAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ""
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if tokenStr == "" {
			cookie, err := r.Cookie("token")
			if err == nil {
				tokenStr = cookie.Value
			}
		}

		if tokenStr != "" {
			claims, err := ValidateJWT(tokenStr)
			if err == nil {
				userID := int(claims["user_id"].(float64))
				user, err := GetUserByID(userID)
				if err == nil {
					ctx := context.WithValue(r.Context(), userContextKey, user)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	}
}

func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(userContextKey).(*User)
		if !user.IsAdmin {
			http.Error(w, `{"error":"Admin access required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getUserFromContext(r *http.Request) *User {
	if user, ok := r.Context().Value(userContextKey).(*User); ok {
		return user
	}
	return nil
}

func generatePassword(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

func ensureAdmin() {
	adminUser := os.Getenv("ADMIN_USERNAME")
	if adminUser == "" {
		adminUser = "admin"
	}
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@shortly.local"
	}

	if _, err := GetUserByUsername(adminUser); err == nil {
		return
	}

	password := generatePassword(16)
	hash, err := HashPassword(password)
	if err != nil {
		fmt.Println("Failed to hash admin password:", err)
		return
	}

	user, err := CreateUser(adminUser, adminEmail, hash)
	if err != nil {
		fmt.Println("Failed to create admin user:", err)
		return
	}

	if err := SetUserAdmin(user.ID, true); err != nil {
		fmt.Println("Failed to set admin role:", err)
		return
	}

	fmt.Println("========================================")
	fmt.Println("  Admin account created on first run")
	fmt.Printf("  Username: %s\n", adminUser)
	fmt.Printf("  Password: %s\n", password)
	fmt.Println("  Change this password after first login!")
	fmt.Println("========================================")
}
