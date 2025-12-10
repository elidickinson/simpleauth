# Simple Auth

This is a stateless forward-auth provider.
I tested it with Caddy, but it should work fine with Traefik.

# Theory of Operation

This issues cryptographically signed authentication tokens to the client.
JavaScript stores the token in an HTTP-only cookie.

When a client presents an authentication token in a cookie,
they are allowed in if the token was properly signed,
and has not expired.

Authentication tokens consist of:

* Username
* Expiration date
* Hashed Message Authentication Code (HMAC)

Simpleauth also works with HTTP Basic authentication and provides a built-in login form.

# Setup

Simpleauth needs two (2) files:

* A secret key, to sign authentication tokens
* A list of usernames and hashed passwords


## Create secret key

**Option 1: File-based (recommended for local deployments)**

This will use `/dev/urandom` to generate a 64-byte secret key.

```sh
SASECRET=/run/secrets/simpleauth.key  # Set to wherever you want your secret to live
dd if=/dev/urandom of=$SASECRET bs=1 count=64
```

**Option 2: Environment variable (ideal for container platforms like Dokploy)**

Generate a base64-encoded secret:

```sh
# Generate once and copy to your platform's secret management
openssl rand -base64 64
```

This will output a base64 string like `exampleBase64SecretHere...` that you can set as the `SIMPLEAUTH_SECRET` environment variable.


## Create password file

**Option 1: File-based**

It's just a text file with hashed passwords.
Each line is of the format `username:password_hash`

```sh
alias sacrypt="docker run --rm --entrypoint=/crypt git.woozle.org/neale/simpleauth"
SAPASSWD=/run/secrets/passwd   # Set to wherever you want your password file to live
: > $SAPASSWD                  # Reset password file
sacrypt user1 password1 >> $SAPASSWD
sacrypt user2 password2 >> $SAPASSWD
sacrypt user3 password3 >> $SAPASSWD
```

**Option 2: Environment variable (ideal for container platforms)**

Set the `SIMPLEAUTH_USERS` environment variable with format `user1:password1,user2:password2`:

```bash
SIMPLEAUTH_USERS="admin:secretpassword,user1:anotherpassword"
```

The passwords will be automatically hashed when loaded.


## Start it

**Option 1: Docker with files**

```sh
docker run \
  --name=simpleauth \
  --detach \
  --restart=always \
  --port 8080:8080 \
  --volume $SASECRET:/run/secrets/simpleauth.key:ro \
  --volume $SAPASSWD:/run/secrets/passwd:ro \
  git.woozle.org/neale/simpleauth
```

**Option 2: Environment variables (great for Dokploy)**

```sh
docker run \
  --name=simpleauth \
  --detach \
  --restart=always \
  --port 8080:8080 \
  -e SIMPLEAUTH_SECRET="your-base64-secret-here" \
  -e SIMPLEAUTH_USERS="admin:password1,user2:password2" \
  -e SIMPLEAUTH_LISTEN=":8080" \
  git.woozle.org/neale/simpleauth
```

## Dokploy Deployment

Dokploy makes deployment simple with environment variables:

1. **Create a new application** in Dokploy
2. **Set these environment variables:**
   - `SIMPLEAUTH_SECRET`: Your base64-encoded 64-byte secret (generate with `openssl rand -base64 64`)
   - `SIMPLEAUTH_USERS`: Your users in format `user1:password1,user2:password2`
   - `SIMPLEAUTH_LISTEN`: `:8080` (or your preferred port)
   - `SIMPLEAUTH_COOKIE_NAME`: Custom cookie name (optional, defaults to `__Http-simpleauth-token`)
3. **Deploy the application** using the Docker image: `git.woozle.org/neale/simpleauth`

**Example Dokploy environment setup:**
```
SIMPLEAUTH_SECRET = SGVsbG9Xb3JsZEhlbGxvV29ybGQxMjM0NTY3ODkwYWJjZGVmZ2hpams=...
SIMPLEAUTH_USERS = admin:securePassword123,developer:devPass456
SIMPLEAUTH_LISTEN = :8080
```

The health endpoint is available at `/health` for monitoring your deployment status.

## Authentication Flow

Simpleauth uses clear HTTP status codes to indicate authentication state:

| Status Code | Authentication State | What Happens |
|-------------|----------------------|--------------|
| **200** | Valid token or basic auth | Forward proxy continues to destination |
| **418** | Login form success | Browser receives Set-Cookie, reloads page |
| **401** | Authentication failed | Shows login form or returns error |

**Flow:**
1. **First request** → No cookie → 401 + login form
2. **Login submit** → Form POST → 418 + Set-Cookie if credentials valid
3. **Browser reload** → Has cookie → 200 → Access granted
4. **Cookie expires** → Back to step 1

The built-in login form automatically handles the cookie flow and provides user feedback on failed attempts.

### Why We Use HTTP 418 for Login Success

Simpleauth returns HTTP 418 for successful login. The code must not be 200 OK or any 2XX status - forward auth proxies would forward the request to the destination without returning the Set-Cookie header to the browser. This would create an infinite authentication loop since the cookie is never set.

HTTP 418 won't be confused with other codes while still allowing the browser to receive the Set-Cookie header.

## Make your web server use it

### Caddy

You'll want a `forward-auth` section like this:

```
private.example.com {
  forward_auth localhost:8080 {
    uri /
    copy_headers X-Simpleauth-Username
  }
  respond "Hello, friend!"
}
```

The `copy_headers` directive tells Caddy to pass
Simpleauth's `X-Simpleauth-Username` header
along in the HTTP request.
If you are reverse proxying to some other app,
it can look at this header to determine who's logged in.

**Optional: Domain-scoped cookies**

If you want the authentication cookie to work across multiple subdomains
(e.g., both `app.example.com` and `api.example.com`), add the `header_up` directive:

```
private.example.com {
  forward_auth localhost:8080 {
    uri /
    copy_headers X-Simpleauth-Username
    header_up X-Simpleauth-Domain example.com    # Share cookie across all of example.com
  }
  respond "Hello, friend!"
}
```

This tells simpleauth to set the cookie's domain attribute,
allowing it to be shared across all subdomains of `example.com`.
Without this, the cookie is scoped only to the specific hostname.

### Traefik

I need someone to send me equivalent
traefik
configuration,
to include here.


### nginx

I need someone to send me equivalent
nginx
configuration,
to include here.


# Why not some other thing?

The main reason is that I couldn't get the freedesktop.org
WebDAV client code to work with anything else I found.

* Authelia - I like it, but I couldn't get WebDAV to work. Also, it used 4.8GB of RAM and wanted a Redis server.
* Authentik - Didn't try it, looked too complicated.
* Keycloak - Didn't try it, looked way too complicated.


# Todo

* [ ] Performance testing: somehow this takes more CPU than caddy?

# Project Home

The canonical home for this project is
https://git.woozle.org/neale/simpleauth

