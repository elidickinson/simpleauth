package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"git.woozle.org/neale/simpleauth/pkg/token"
	"github.com/GehirnInc/crypt"
	_ "github.com/GehirnInc/crypt/sha256_crypt"
)

const CookieName = "simpleauth-token"

var secret []byte

// getEnvWithFallback returns environment value or fallback to default
func getEnvWithFallback(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getSecret loads secret from environment variable or file
func getSecret(secretPath string) ([]byte, error) {
	// Try environment variable first
	if secretEnv := os.Getenv("SIMPLEAUTH_SECRET"); secretEnv != "" {
		decodedSecret, err := base64.StdEncoding.DecodeString(secretEnv)
		if err != nil {
			return nil, fmt.Errorf("invalid SIMPLEAUTH_SECRET: %w", err)
		}
		if len(decodedSecret) < 64 {
			return nil, fmt.Errorf("SIMPLEAUTH_SECRET must be at least 64 bytes (got %d)", len(decodedSecret))
		}
		return decodedSecret[:64], nil
	}

	// Try to read from file
	if _, err := os.Stat(secretPath); err != nil {
		return nil, fmt.Errorf("secret not configured (no SIMPLEAUTH_SECRET env var and no file at %s): %w", secretPath, err)
	}

	content, err := ioutil.ReadFile(secretPath)
	if err != nil {
		return nil, err
	}
	if len(content) < 64 {
		return nil, fmt.Errorf("secret file at %s must be at least 64 bytes (got %d)", secretPath, len(content))
	}
	return content[:64], nil
}

// loadPasswordsFromEnv loads passwords from SIMPLEAUTH_USERS env var
// Format: SIMPLEAUTH_USERS="user1:password1,user2:password2"
func loadPasswordsFromEnv() (map[string]string, error) {
	passwords := make(map[string]string)
	users := os.Getenv("SIMPLEAUTH_USERS")
	if users == "" {
		return passwords, nil
	}

	pairs := strings.Split(users, ",")
	for _, pair := range pairs {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			log.Printf("Warning: invalid user format '%s', expected 'username:password'", pair)
			continue
		}
		username := strings.TrimSpace(parts[0])
		hash := strings.TrimSpace(parts[1])
		passwords[username] = hash
	}
	return passwords, nil
}

// getPasswords loads passwords from file or environment variable
func getPasswords(passwordPath string, usersEnv string) (map[string]string, error) {
	// If environment variable is set, use it
	if usersEnv != "" {
		passwords, err := loadPasswordsFromEnv()
		if err != nil {
			return nil, err
		}
		return passwords, nil
	}

	// Otherwise use password file
	if _, err := os.Stat(passwordPath); err != nil {
		return nil, err
	}

	f, err := os.Open(passwordPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	passwords := make(map[string]string)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) >= 2 {
			username := parts[0]
			hash := parts[1]
			passwords[username] = hash
		}
	}
	return passwords, scanner.Err()
}

var startTime = time.Now()
var lifespan time.Duration
var cryptedPasswords map[string]string
var loginHtml []byte
var verbose bool

func debugln(v ...any) {
	if verbose {
		log.Println(v...)
	}
}

func debugf(fmt string, v ...any) {
	if verbose {
		log.Printf(fmt, v...)
	}
}

func authenticationValid(username, password string) bool {
	c := crypt.SHA256.New()
	if crypted, ok := cryptedPasswords[username]; ok {
		debugf("verifying password for username:%v against hash:%v", username, crypted)
		if err := c.Verify(crypted, []byte(password)); err == nil {
			debugf("password verification succeeded for username:%v", username)
			return true
		} else {
			debugf("password verification failed for username:%v error:%v", username, err)
			if strings.Contains(err.Error(), "invalid salt") {
				debugf("INVALID SALT FORMAT: This usually means dollar signs in hash were not wrapped in single quotes in the environment variable")
			}
		}
	} else {
		debugf("no hash found for username:%v", username)
	}
	return false
}

func usernameIfAuthenticated(req *http.Request) string {
	if authUsername, authPassword, ok := req.BasicAuth(); ok {
		valid := authenticationValid(authUsername, authPassword)
		debugf("basic auth valid:%v username:%v", valid, authUsername)
		if valid {
			return authUsername
		}
	} else {
		debugf("no basic auth")
	}

	ncookies := 0
	for i, cookie := range req.Cookies() {
		if cookie.Name != CookieName {
			continue
		}
		t, _ := token.ParseString(cookie.Value)
		valid := t.Valid(secret)
		debugf("cookie %d valid:%v username:%v", i, valid, t.Username)
		if valid {
			return t.Username
		}
		ncookies += 1
	}
	if ncookies == 0 {
		debugf("no cookies")
	}

	return ""
}

func rootHandler(w http.ResponseWriter, req *http.Request) {
	var status string
	username := usernameIfAuthenticated(req)
	login := req.Header.Get("X-Simpleauth-Login") == "true"
	browser := strings.Contains(req.Header.Get("Accept"), "text/html")

	if username == "" {
		status = "failed"
	} else {
		status = "succeeded"
		w.Header().Set("X-Simpleauth-Username", username)

		if login {
			// Send back a token as a Set-Cookie header
			t := token.New(secret, username, time.Now().Add(lifespan))

			// Build Set-Cookie header with standard attributes
			cookieValue := fmt.Sprintf("%s=%s; Path=/; Secure; HttpOnly; SameSite=Strict; Max-Age=%d",
				CookieName, t.String(), int(lifespan.Seconds()))

			// Add domain if Caddy specified one (via header_up)
			if domain := req.Header.Get("X-Simpleauth-Domain"); domain != "" {
				cookieValue += fmt.Sprintf("; Domain=%s", domain)
			}

			w.Header().Set("Set-Cookie", cookieValue)
		} else {
			// This is the only time simpleauth returns 200
			// That will cause Caddy to proceed with the original request
			http.Error(w, "Success", http.StatusOK)
			return
		}
		// Fall through to the 401 response, though,
		// so that Caddy will send our response back to the client,
		// which needs these headers to set the cookie and try again.
	}

	clientIP := req.Header.Get("X-Real-IP")
	if clientIP == "" {
		clientIP = req.RemoteAddr
	}
	forwardedMethod := req.Header.Get("X-Forwarded-Method")
	forwardedURL := url.URL{
		Scheme: req.Header.Get("X-Forwarded-Proto"),
		Host:   req.Header.Get("X-Forwarded-Host"),
		Path:   req.Header.Get("X-Forwarded-Uri"),
		User:   url.UserPassword(username, ""),
	}

	// Log the request
	if false {
		log.Printf("%s %s %s login:%v %s",
			clientIP, forwardedMethod, forwardedURL.String(),
			login, status,
		)
	}

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("X-Simpleauth-Authentication", status)
	w.Header().Set("WWW-Authenticate", "Simpleauth-Login")
	if !login && !browser {
		// Make browsers use our login form instead of basic auth
		w.Header().Add("WWW-Authenticate", "Basic realm=\"simpleauth\"")
	}

	// Return appropriate status code
	if username != "" && login {
		// Authentication succeeded in login mode - return 418 with Set-Cookie
		w.WriteHeader(http.StatusTeapot)
	} else {
		// Authentication failed - return 401
		w.WriteHeader(http.StatusUnauthorized)
	}

	w.Write(loginHtml)
}

// healthHandler returns health status for monitoring
func healthHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Check if we have users and secret configured
	status := map[string]interface{}{
		"status":     "healthy",
		"users":      len(cryptedPasswords),
		"secret_set": len(secret) >= 64,
		"uptime":     time.Since(startTime).String(), // Actual uptime
	}

	// If no users configured, mark as unhealthy
	if len(cryptedPasswords) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		status["status"] = "unhealthy"
		status["error"] = "no users configured"
	}

	// If secret not configured, mark as unhealthy
	if len(secret) < 64 {
		w.WriteHeader(http.StatusServiceUnavailable)
		status["status"] = "unhealthy"
		status["error"] = "secret not properly configured"
	}

	json.NewEncoder(w).Encode(status)
}

func main() {
	// Support both flags and environment variables
	listen := flag.String(
		"listen",
		getEnvWithFallback("SIMPLEAUTH_LISTEN", ":8080"),
		"Bind address for incoming HTTP connections",
	)
	lifespanStr := flag.String(
		"lifespan",
		getEnvWithFallback("SIMPLEAUTH_LIFESPAN", "2400h"),
		"How long an issued token is valid (e.g., 100h, 30d)",
	)
	passwordPath := flag.String(
		"passwd",
		getEnvWithFallback("SIMPLEAUTH_PASSWORD_FILE", "/run/secrets/passwd"),
		"Path to a file containing passwords",
	)
	secretPath := flag.String(
		"secret",
		getEnvWithFallback("SIMPLEAUTH_SECRET_FILE", "/run/secrets/simpleauth.key"),
		"Path to a file containing some sort of secret, for signing requests",
	)
	htmlPath := flag.String(
		"html",
		getEnvWithFallback("SIMPLEAUTH_HTML_PATH", "web"),
		"Path to HTML files",
	)
	flag.BoolVar(
		&verbose,
		"verbose",
		os.Getenv("SIMPLEAUTH_VERBOSE") == "true",
		"Print verbose logs, for debugging",
	)
	flag.Parse()

	// Parse lifespan duration
	var err error
	lifespan, err = time.ParseDuration(*lifespanStr)
	if err != nil {
		log.Fatalf("Invalid lifespan duration: %v", err)
	}

	// Load passwords from file or environment
	usersEnv := os.Getenv("SIMPLEAUTH_USERS")
	cryptedPasswords, err = getPasswords(*passwordPath, usersEnv)
	if err != nil {
		log.Fatal(err)
	}

	// Load HTML
	loginHtml, err = ioutil.ReadFile(path.Join(*htmlPath, "login.html"))
	if err != nil {
		log.Fatal(err)
	}

	// Load secret from environment variable or file
	secret, err = getSecret(*secretPath)
	if err != nil {
		log.Fatal(err)
	}

	if verbose {
		log.Printf("Loaded %d users", len(cryptedPasswords))
		if usersEnv != "" {
			log.Println("Using environment variable for users")
		} else {
			log.Printf("Using password file: %s", *passwordPath)
		}
		if os.Getenv("SIMPLEAUTH_SECRET") != "" {
			log.Println("Using SIMPLEAUTH_SECRET environment variable")
		} else {
			log.Printf("Using secret file: %s", *secretPath)
		}
	}

	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/health", healthHandler)

	fmt.Println("listening on", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}
