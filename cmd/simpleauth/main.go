package main

import (
	"bufio"
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

var secret []byte = make([]byte, 256)
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
		if err := c.Verify(crypted, []byte(password)); err == nil {
			return true
		}
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
	w.WriteHeader(http.StatusUnauthorized)
	w.Write(loginHtml)
}

func main() {
	listen := flag.String(
		"listen",
		":8080",
		"Bind address for incoming HTTP connections",
	)
	flag.DurationVar(
		&lifespan,
		"lifespan",
		100*24*time.Hour,
		"How long an issued token is valid",
	)
	passwordPath := flag.String(
		"passwd",
		"/run/secrets/passwd",
		"Path to a file containing passwords",
	)
	secretPath := flag.String(
		"secret",
		"/run/secrets/simpleauth.key",
		"Path to a file containing some sort of secret, for signing requests",
	)
	htmlPath := flag.String(
		"html",
		"web",
		"Path to HTML files",
	)
	flag.BoolVar(
		&verbose,
		"verbose",
		false,
		"Print verbose logs, for debugging",
	)
	flag.Parse()

	cryptedPasswords = make(map[string]string, 10)
	if f, err := os.Open(*passwordPath); err != nil {
		log.Fatal(err)
	} else {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				username := parts[0]
				password := parts[1]
				cryptedPasswords[username] = password
			}
		}
	}

	var err error

	loginHtml, err = ioutil.ReadFile(path.Join(*htmlPath, "login.html"))
	if err != nil {
		log.Fatal(err)
	}

	// Read in secret
	f, err := os.Open(*secretPath)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	l, err := f.Read(secret)
	if l < 64 {
		log.Fatalf("Secret file provided %d bytes. That's not enough bytes!", l)
	} else if err != nil {
		log.Fatal(err)
	}
	secret = secret[:l]

	http.HandleFunc("/", rootHandler)

	fmt.Println("listening on", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}
