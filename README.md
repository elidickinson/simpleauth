# Simple Auth

This is a stateless forward-auth provider.
I tested it with Caddy, but it should work fine with Traefik.

# Theory of Operation

This issues cryptographically signed authentication tokens to the client.
Some JavaScript stores the token in a cookie.

When a client presents an authentication token in a cookie,
they are allowed in if the token was properly signed,
and has not expired.

Authentication tokens consist of:

* Username
* Expiration date
* Hashed Message Authentication Code (HMAC)

Simpleauth also works with HTTP Basic authentication.

# Setup

Simpleauth needs two (2) files:

* A secret key, to sign authentication tokens
* A list of usernames and hashed passwords


## Create secret key

This will use `/dev/urandom` to generate a 64-byte secret key.

```sh
SASECRET=/run/secrets/simpleauth.key  # Set to wherever you want your secret to live
dd if=/dev/urandom of=$SASECRET bs=1 count=64
```


## Create password file

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


## Start it

Turning this into the container orchestration system you prefer
(Docker Swarm, Kubernetes, Docker Compose)
is left as an exercise for the reader.

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

