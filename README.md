# Shortly - URL Shortener

Shortly is a lightweight and efficient URL shortener built using Go. It allows you to shorten URLs, which can then be easily shared. The app also handles redirecting users to the original URL and serves a custom 404 page when a short URL is not found.

> This Project is totaly for fun, there is a [todo](todo.md) list if you want to contribute on this project. 

## Features

- Shorten long URLs into easily shareable short links.
- Redirect users to the original URL when visiting a short URL.
- Serve a custom 404 page when the short URL does not exist.

## Prerequisites

Before you begin, ensure that you have the following installed:

- Go 1.18 or later
- Git

## Setup

1. **Clone the repository:**

   ```bash
   git clone https://github.com/behdanisohrab/shortly.git
   cd shortly
   ```

2. **Install dependencies:**

   Use the following command to install the required Go dependencies:

   ```bash
   go mod tidy
   ```

3. **Configuration:**

   Rename the `.env.example` file to `.env` and configure it with the following environment variables:

   ```env
   DOMAIN=localhost
   PORT=8080
   ```

   - Set `DOMAIN` to your domain if deploying to production.
   - Set `PORT` to the port number you want the application to listen on.

4. **Build the application:**

   Run the following command to build the Go application:

   ```bash
   go build -o app .
   ```

5. **Run the application:**

   Start the server with:

   ```bash
   ./app
   ```

   The application will be accessible at `http://localhost:8080/` or the domain/port you configured.

## API

Once the application is running, you can interact with the API to create short URLs.

### Create Short URL

- **Endpoint:** `/create`
- **Method:** `POST`
- **Request body (JSON):**

  ```json
  {
    "url": "https://example.com"
  }
  ```

- **Response (JSON):**

  ```json
  {
    "short_url": "http://localhost:8080/abc123"
  }
  ```

#### Example `curl` Request

You can use `curl` to create a short URL by sending a POST request with a URL:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"url":"https://example.com"}' http://localhost:8080/create
```

A successful response will return a short URL:

```json
{
  "short_url": "http://localhost:8080/abc123"
}
```

### Redirect to Original URL

When users visit the short URL (e.g., `http://localhost:8080/abc123`), they will be automatically redirected to the original URL (e.g., `https://example.com`).

### 404 Handling

If a non-existent short URL is requested, the application will serve a custom `404.html` page indicating that the URL was not found.




