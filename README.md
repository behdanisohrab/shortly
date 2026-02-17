 Shortly - URL Shortener


Shortly is a feature-rich URL shortener built with Go. It supports user authentication, click analytics, custom short codes, QR code generation, link expiration, rate limiting, an admin dashboard, and more.

> [!NOTE]
> This project is totally for fun, there is a [todo](todo.md) list if you want to contribute on this project. 

## Features

- Shorten long URLs into easily shareable short links
- User registration and login with JWT authentication
- Click analytics and tracking (IP, user agent, referer, timestamp)
- Custom short codes (e.g., `http://short.ly/my-link`)
- Link expiration (time-based and max-use limits)
- QR code generation and download for any short link
- Rate limiting (30 requests/minute per IP)
- Domain blacklisting to block malicious URLs
- Custom redirect types (301 permanent / 302 temporary)
- Admin dashboard for managing URLs, users, and blacklist
- Farsi/Persian UI with dark theme
- Auto-generated admin account on first run

## Prerequisites

- Go 1.20 or later
- Git
- GCC (required for SQLite CGO compilation)

## Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/behdanisohrab/shortly.git
   cd shortly
   ```

2. **Install dependencies:**

   ```bash
   go mod tidy
   ```

3. **Configuration:**

   Copy `.env.example` to `.env` and configure it:

   ```bash
   cp .env.example .env
   ```

   ```env
   DOMAIN=localhost
   PORT=8080
   JWT_SECRET=change-this-to-a-random-secret-string
   ADMIN_USERNAME=admin
   ADMIN_EMAIL=admin@shortly.local
   ```

   - `DOMAIN` - Your domain name (use `localhost` for development)
   - `PORT` - Server port (default: `8080`)
   - `JWT_SECRET` - Secret key for JWT token signing (change this in production!)
   - `ADMIN_USERNAME` - Admin account username (default: `admin`)
   - `ADMIN_EMAIL` - Admin account email (default: `admin@shortly.local`)

4. **Build the application:**

   ```bash
   go build -o shortly .
   ```

5. **Run the application:**

   ```bash
   ./shortly
   ```

   The application will be accessible at `http://localhost:8080/`.

## API

All API endpoints are under the `/api/` prefix. Authenticated endpoints require a `Authorization: Bearer <token>` header or a `token` cookie.

### Authentication

| Endpoint | Method | Auth | Description |
|---|---|---|---|
| `/api/auth/register` | POST | No | Register a new user |
| `/api/auth/login` | POST | No | Login and get JWT token |
| `/api/auth/logout` | POST | No | Clear auth cookie |
| `/api/auth/me` | GET | Yes | Get current user info |

#### Register

```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"myuser","email":"user@example.com","password":"mypassword"}'
```

#### Login

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"myuser","password":"mypassword"}'
```

Response:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": {"id": 1, "username": "myuser", "email": "user@example.com", "is_admin": false}
}
```

### URL Shortening

| Endpoint | Method | Auth | Description |
|---|---|---|---|
| `/api/urls/create` | POST | Optional | Create a short URL |
| `/api/urls/my` | GET | Yes | List your URLs with click counts |
| `/api/urls/delete?id=` | DELETE | Yes | Delete one of your URLs |
| `/api/urls/stats?code=` | GET | No | Get click stats for a URL |

#### Create Short URL

```bash
curl -X POST http://localhost:8080/api/urls/create \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com"}'
```

With all options:

```bash
curl -X POST http://localhost:8080/api/urls/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "url": "https://example.com",
    "custom_code": "my-link",
    "redirect_type": 301,
    "expires_in": 24,
    "max_uses": 100
  }'
```

| Field | Type | Description |
|---|---|---|
| `url` | string | **(required)** The URL to shorten |
| `custom_code` | string | Custom short code (3-20 chars) |
| `redirect_type` | int | `301` (permanent) or `302` (temporary, default) |
| `expires_in` | int | Hours until expiration (0 = never) |
| `max_uses` | int | Maximum number of redirects (0 = unlimited) |

Response:

```json
{
  "short_url": "http://localhost:8080/my-link",
  "code": "my-link",
  "redirect_type": 301,
  "expires_at": "2026-02-18T12:00:00Z",
  "max_uses": 100
}
```

### QR Code

| Endpoint | Method | Description |
|---|---|---|
| `/api/qr?code=abc123&size=256` | GET | Returns a PNG QR code image |

```bash
curl -o qr.png "http://localhost:8080/api/qr?code=abc123&size=512"
```

The `size` parameter accepts values from 64 to 1024 (default: 256).

### Admin Endpoints

All admin endpoints require authentication with an admin user.

| Endpoint | Method | Description |
|---|---|---|
| `/api/admin/stats` | GET | Get system-wide statistics |
| `/api/admin/urls?limit=50&offset=0` | GET | List all URLs |
| `/api/admin/users?limit=50&offset=0` | GET | List all users |
| `/api/admin/urls/delete?id=` | DELETE | Delete any URL |
| `/api/admin/blacklist` | GET | List blacklisted domains |
| `/api/admin/blacklist` | POST | Add domain to blacklist |
| `/api/admin/blacklist?domain=` | DELETE | Remove domain from blacklist |
| `/api/admin/set-admin` | POST | Set/remove admin role for a user |

#### Blacklist a domain

```bash
curl -X POST http://localhost:8080/api/admin/blacklist \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{"domain":"malicious-site.com","reason":"Phishing"}'
```

### Redirect

Visiting `http://localhost:8080/{code}` redirects to the original URL (using the configured redirect type). Expired and maxed-out links return a 404 page.

### Health Check

```bash
curl http://localhost:8080/api/health
```

## Testing with curl

Here's a quick workflow to test the app end-to-end:

```bash
# 1. Start the server
go run . &

# 2. Create a short URL (anonymous)
curl -s -X POST http://localhost:8080/api/urls/create \
  -H "Content-Type: application/json" \
  -d '{"url":"https://github.com"}' | jq

# 3. Register a user
curl -s -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@test.com","password":"password123"}' | jq

# 4. Save the token from step 3, then create an authenticated short URL
TOKEN="<paste token here>"
curl -s -X POST http://localhost:8080/api/urls/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"url":"https://go.dev","custom_code":"golang","expires_in":48,"max_uses":10}' | jq

# 5. View your URLs
curl -s http://localhost:8080/api/urls/my \
  -H "Authorization: Bearer $TOKEN" | jq

# 6. Get stats for a link
curl -s "http://localhost:8080/api/urls/stats?code=golang" | jq

# 7. Download a QR code
curl -o qr.png "http://localhost:8080/api/qr?code=golang&size=512"

# 8. Test the redirect
curl -v http://localhost:8080/golang
```

## Admin Account

On first run, Shortly automatically creates an admin account and prints the generated password to the server log:

```
========================================
  Admin account created on first run
  Username: admin
  Password: a1b2c3d4e5f6g7h8
  Change this password after first login!
========================================
```

You can customize the admin username and email via environment variables (`ADMIN_USERNAME`, `ADMIN_EMAIL`).

## Database

Shortly uses SQLite by default. The database file `shortly.db` is created automatically in the project root. No external database setup is required.

## License

[GNU AGPL v3](LICENSE)
