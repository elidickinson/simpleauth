# Dokploy Deployment Guide

This guide shows how to deploy Simpleauth on Dokploy using environment variables - the recommended and simplest approach.

## Quick Start

Deploy Simpleauth on Dokploy in minutes with environment variables:

### 1. Create New Application

In Dokploy:
- Click "Create Application"
- Choose "Docker Image"
- Image: `git.woozle.org/neale/simpleauth`
- Tag: `latest`

### 2. Configure Environment Variables

Add these environment variables in the Dokploy application settings:

```bash
# Secret key (REQUIRED - generate with: openssl rand -base64 64)
SIMPLEAUTH_SECRET=your-generated-base64-secret-here

# Users (REQUIRED - format: user1:password1,user2:password2)
SIMPLEAUTH_USERS=admin:yourSecurePassword123,user:anotherPass

# Server (optional - defaults shown)
SIMPLEAUTH_LISTEN=:8080
SIMPLEAUTH_VERBOSE=true

# Token lifespan (optional - e.g., 168h = 7 days)
SIMPLEAUTH_LIFESPAN=168h
```

### 3. Configure Port Mapping

- Container Port: `8080`
- Host Port: Choose your desired port (e.g., `8080`)

### 4. Deploy and Test

Click "Deploy" and test by visiting your deployment URL. Dokploy will automatically:
- Pull the Docker image
- Deploy the container with your environment variables

## Environment Variables Reference

| Variable | Default | Required? | Description | Example |
|----------|---------|-----------|-------------|---------|
| `SIMPLEAUTH_SECRET` | (none) | **Yes** | Base64-encoded secret key | Generate with `openssl rand -base64 64` |
| `SIMPLEAUTH_USERS` | (none) | No* | Users (format: user:pass,...) | `admin:secret123,user:pass456` |
| `SIMPLEAUTH_LISTEN` | `:8080` | No | Bind address | `:8080` or `0.0.0.0:8080` |
| `SIMPLEAUTH_LIFESPAN` | `2400h` (100 days) | No | Token validity period | `24h`, `7d`, `168h` |
| `SIMPLEAUTH_PASSWORD_FILE` | `/run/secrets/passwd` | No | Path to password file | `/etc/simpleauth/passwd` |
| `SIMPLEAUTH_SECRET_FILE` | `/run/secrets/simpleauth.key` | No | Path to secret file | `/etc/simpleauth/key` |
| `SIMPLEAUTH_HTML_PATH` | `web` | No | Path to HTML templates | `/var/www/html` |
| `SIMPLEAUTH_VERBOSE` | `false` | No | Enable debug logs | `true` |

*Required unless `SIMPLEAUTH_PASSWORD_FILE` is provided

**Required Variables:** You must set `SIMPLEAUTH_SECRET` and either `SIMPLEAUTH_USERS` or `SIMPLEAUTH_PASSWORD_FILE` for the application to start.

**Password Format:** Use plaintext passwords (e.g., `admin:password123,user:pass456`). Simpleauth automatically hashes them when loaded.

**Hash Format Reference:** For troubleshooting, SHA256 password hashes look like: `admin:$5$rounds=535000$salt$hash`

## Domain Scoping (Multi-site Authentication)

If you want the auth cookie to work across multiple subdomains:

1. **In your reverse proxy (Caddy/NGINX), set a header:**
```
# For Caddy
header_up X-Simpleauth-Domain example.com

# For NGINX  
proxy_set_header X-Simpleauth-Domain example.com;
```

2. **No additional Dokploy configuration needed** - Simpleauth will read this from the upstream header automatically.

## Health Monitoring

Simpleauth includes a built-in health check at `/health`:

```bash
curl http://your-app:8080/health
# Response: {"status":"healthy","users":2,"secret_set":true}
```

Docker health check is automatically configured (every 30s).

## Production Best Practices

- **Strong passwords**: `admin:TrulyStrongPassword123!@,user:AnotherGoodPass456`
- **Token lifespan**: 168h (7 days) for standard, 24h for high security, 4h for maximum
- **Secret management**: Generate with `openssl rand -base64 64`, store in Dokploy env vars only
- **Logging**: Set `SIMPLEAUTH_VERBOSE=true` for troubleshooting, `false` for production

## Troubleshooting

### Can't Login

1. Check health endpoint: `GET /health`
2. Ensure `SIMPLEAUTH_USERS` is set correctly in Dokploy environment variables
3. Verify format: `username:password,user2:password2` (spaces are trimmed automatically)
4. Check that passwords contain only allowed characters

### Secret Configuration Issues

1. Ensure `SIMPLEAUTH_SECRET` environment variable is set in Dokploy
2. Verify the secret was generated with `openssl rand -base64 64`
3. Secret must be valid base64 string
4. Check there are no typos in the variable name

### Health Check Shows "unhealthy"

Check the health endpoint response:
- `{"status":"unhealthy","error":"no users configured"}` → Set `SIMPLEAUTH_USERS`
- `{"status":"unhealthy","error":"secret not properly configured"}` → Set `SIMPLEAUTH_SECRET` correctly

### Common Issues

- **Variable typos**: Ensure `SIMPLEAUTH_USERS` and `SIMPLEAUTH_SECRET` are spelled exactly
- **Format errors**: Users format is `user:pass,user2:pass2` (spaces are trimmed automatically)
- **Restart required**: After changing environment variables, restart the application

**Migration from password files:** Simply replace hash entries with plaintext passwords in `SIMPLEAUTH_USERS`. No conversion needed - just use the actual passwords.