package main

import (
    "encoding/json"
    "fmt"
    "math/rand"
    "net/http"
    "net/url"
    "os"
    "time"

    "github.com/joho/godotenv"
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
        fmt.Println("Error loading .env file")
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
}

func generateRandomCode() string {
    b := make([]byte, codeLength)
    for i := range b {
        b[i] = charset[rand.Intn(len(charset))]
    }
    return string(b)
}

func createShortURL(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var request struct {
        URL string `json:"url"`
    }

    if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    if request.URL == "" {
        http.Error(w, "URL is required", http.StatusBadRequest)
        return
    }

    if _, err := url.ParseRequestURI(request.URL); err != nil {
        http.Error(w, "Invalid URL", http.StatusBadRequest)
        return
    }

    var code string
    for {
        code = generateRandomCode()
        exists, _ := URLExists(code)
        if !exists {
            break
        }
    }

    if err := SaveURL(code, request.URL); err != nil {
        http.Error(w, "Error saving URL", http.StatusInternalServerError)
        return
    }

    var shortURL string
    if domain == "localhost" {
        shortURL = fmt.Sprintf("http://%s:%s/%s", domain, port, code) 
    } else {
        shortURL = fmt.Sprintf("http://%s/%s", domain, code) 
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "short_url": shortURL,
    })
}

func redirectShortURL(w http.ResponseWriter, r *http.Request) {
    code := r.URL.Path[1:]

    if code == "" {
        http.ServeFile(w, r, "static/index.html")
        return
    }

    originalURL, err := GetURL(code)
    if err != nil {
        http.ServeFile(w, r, "static/404.html") 
        return
    }

    http.Redirect(w, r, originalURL, http.StatusFound)
}

func main() {
    http.HandleFunc("/", redirectShortURL)
    http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
    http.HandleFunc("/create", createShortURL)
    fmt.Printf("Shortly server started on http://%s:%s\n", domain, port) 
    http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}

