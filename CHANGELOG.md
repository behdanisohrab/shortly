# Changelog

## [0.2.0] - 2026-02-17

Major rewrite from the original single-endpoint URL shortener to a full-featured platform.

### Added

**Authentication & Users**
- User registration and login system with JWT tokens
- Password hashing with bcrypt
- Cookie-based and header-based token authentication
- Admin role system with middleware protection
- Auto-generated admin account on first run (password printed to server log)
- Configurable admin username/email via `ADMIN_USERNAME` and `ADMIN_EMAIL` env vars

**URL Management**
- Custom short codes (user-defined, 3-20 characters)
- URL expiration (time-based, configurable in hours)
- Maximum usage limits per short URL
- Custom redirect types (301 permanent / 302 temporary)
- Reserved code protection (prevents conflicts with API routes)
- URL validation requiring `http://` or `https://` scheme

**Analytics**
- Click tracking per URL (IP, user agent, referer, timestamp)
- Per-URL statistics API endpoint
- Click count aggregation
- Daily click breakdown for admin dashboard

**QR Codes**
- QR code generation for any short URL
- Configurable image size (64-1024px)
- Download support

**Rate Limiting**
- In-memory rate limiter (30 requests/minute per IP)
- Automatic cleanup of stale visitor records
- `X-Forwarded-For` and `X-Real-IP` header support

**Security**
- Domain blacklisting system
- Blacklist management API (add/remove/list)
- URL domain extraction and validation against blacklist
- CORS middleware

**Admin Dashboard**
- System-wide statistics (total URLs, clicks, users)
- URL management (view all, delete any)
- User management (view all, toggle admin role)
- Blacklist management (add/remove domains)

**Infrastructure**
- SQLite WAL mode with busy timeout for better concurrency
- Database indexes on frequently queried columns
- Automatic cleanup of expired and maxed-out URLs (every 5 minutes)
- Health check endpoint (`/api/health`)
- `.gitignore` file

### Changed

- **API routes moved under `/api/` prefix** (was `/create`, now `/api/urls/create`)
- **Database schema expanded** from single `urls` table to `users`, `urls`, `clicks`, `blacklist`
- **`urls` table** now includes `user_id`, `custom_code`, `redirect_type`, `expires_at`, `max_uses`, `use_count`
- **URL creation response** now returns `code`, `redirect_type`, `expires_at`, `max_uses` alongside `short_url`
- **Redirect handler** now checks expiration, max uses, increments use count, and records clicks
- **`.env.example`** updated with `JWT_SECRET`, `ADMIN_USERNAME`, `ADMIN_EMAIL`
- **Go version** updated from 1.20 to 1.24

**UI/Frontend**
- Complete rewrite of `index.html` as a single-page application
- Full Farsi/Persian interface
- Dark theme with clean, minimal design
- Fixed logout button showing when not logged in
- Removed excessive animations and gradients
- Added user dashboard page (view/delete own URLs, view stats)
- Added admin panel page (stats, URLs, users, blacklist management)
- Added login/register modals
- Added QR code preview and download
- Advanced URL creation options (custom code, redirect type, expiration, max uses)
- Toast notifications for user feedback
- Responsive design for mobile
- Rewritten `404.html` in Farsi with matching dark theme

### Removed
- Direct `/create` endpoint (replaced by `/api/urls/create`)
- Old English/Farsi mixed UI
- Draggable astronaut and UFO from 404 page

### Dependencies Added
- `github.com/golang-jwt/jwt/v5` - JWT authentication
- `github.com/skip2/go-qrcode` - QR code generation
- `golang.org/x/crypto` - bcrypt password hashing

### New Files
- `auth.go` - Authentication, JWT, password hashing, middleware, admin bootstrap
- `ratelimit.go` - Rate limiting middleware
- `CHANGELOG.md` - This file
- `.gitignore`

---

## [0.1.0] - Initial Release

Basic URL shortener with SQLite backend.

- Shorten URLs with random 6-character codes
- Redirect short URLs to originals
- Simple Farsi web interface
- Custom 404 page
- Environment-based configuration (domain, port)
