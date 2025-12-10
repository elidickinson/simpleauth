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

# Users (REQUIRED - format: user1:hash1,user2:hash2)
# Generate hashes with: go run ./cmd/crypt username password
# IMPORTANT: Hash contains $ symbols - see escaping notes below
SIMPLEAUTH_USERS=admin:$5$rounds=535000$salt$hash,user:$5$rounds=535000$salt2$hash2

# Server (optional - defaults shown)
SIMPLEAUTH_LISTEN=:8080
SIMPLEAUTH_VERBOSE=true

# Token lifespan (optional - e.g., 168h = 7 days)
SIMPLEAUTH_LIFESPAN=168h
```

### 3. Important: Escaping Dollar Signs

**CRITICAL:** SHA256 password hashes contain `$` symbols that **MUST** be escaped in Dokploy's environment variable UI.

When entering `SIMPLEAUTH_USERS` in Dokploy:
- Hashes look like: `$5$salt$hash`
- **Wrap the entire value in single quotes** to prevent variable expansion
- Example: `'eli:$5$YqH7sB4YZa7KOuG/$R8TkMFI5wi9BffSHr.8anWVCKPRkEEKM2t6k.jji/v7'`

Without single quotes, the `$` symbols will be interpreted as variable references and your hash will be corrupted, causing authentication to fail with "invalid salt format" errors.

### 4. Configure Port Mapping

- Container Port: `8080`
- Host Port: Choose your desired port (e.g., `8080`)

### 5. Deploy and Test

Click "Deploy" and test by visiting your deployment URL. Dokploy will automatically:
- Pull the Docker image
- Deploy the container with your environment variables

## Environment Variables Reference

| Variable | Default | Required? | Description | Example |
|----------|---------|-----------|-------------|---------|
| `SIMPLEAUTH_SECRET` | (none) | **Yes** | Base64-encoded secret key | Generate with `openssl rand -base64 64` |
| `SIMPLEAUTH_USERS` | (none) | No* | Users (format: user:hash,...) - hashes must be pre-generated | `admin:$5$salt$hash,user:$5$salt2$hash2` (see escaping notes) |
| `SIMPLEAUTH_LISTEN` | `:8080` | No | Bind address | `:8080` or `0.0.0.0:8080` |
| `SIMPLEAUTH_LIFESPAN` | `2400h` (100 days) | No | Token validity period | `24h`, `7d`, `168h` |
| `SIMPLEAUTH_PASSWORD_FILE` | `/run/secrets/passwd` | No | Path to password file | `/etc/simpleauth/passwd` |
| `SIMPLEAUTH_SECRET_FILE` | `/run/secrets/simpleauth.key` | No | Path to secret file | `/etc/simpleauth/key` |
| `SIMPLEAUTH_HTML_PATH` | `web` | No | Path to HTML templates | `/var/www/html` |
| `SIMPLEAUTH_VERBOSE` | `false` | No | Enable debug logs | `true` |

*Required unless `SIMPLEAUTH_PASSWORD_FILE` is provided

**Required Variables:** You must set `SIMPLEAUTH_SECRET` and either `SIMPLEAUTH_USERS` or `SIMPLEAUTH_PASSWORD_FILE` for the application to start.

**Hash Generation:** Use the crypt tool to generate password hashes:
```bash
docker run --rm git.woozle.org/neale/simpleauth:latest /crypt username password
# Or locally: go run ./cmd/crypt username password
```

**Hash Format:** SHA256 password hashes look like: `$5$rounds=535000$salt$hash`

**CRITICAL:** In Dokploy's environment variable UI, wrap the entire value in single quotes to prevent `$` expansion.

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

- **Strong password hashes**: Generate with `/crypt` tool for SHA256 hashing
- **Token lifespan**: 168h (7 days) for standard, 24h for high security, 4h for maximum
- **Secret management**: Generate with `openssl rand -base64 64`, store in Dokploy env vars only
- **Logging**: Set `SIMPLEAUTH_VERBOSE=true` for troubleshooting, `false` for production

## Troubleshooting

### Can't Login / "invalid salt format" error

**Most common cause:** Dollar signs (`$`) in password hashes not escaped properly in Dokploy's environment UI.

1. **Check dollar sign escaping**: Ensure the entire hash value is wrapped in single quotes in Dokploy's UI
   - Wrong: `eli:$5$salt$hash` (unquoted - dollar signs get expanded)
   - Correct: `'eli:$5$salt$hash'` (single quotes prevent expansion)
2. Check health endpoint: `GET /health` to verify users are loaded
3. Ensure `SIMPLEAUTH_USERS` format: `username:hash,user2:hash2` (spaces are trimmed automatically)
4. Verify hashes were generated with `/crypt` tool
5. Enable verbose logging (`SIMPLEAUTH_VERBOSE=true`) and check logs for specific errors

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

- **Dollar sign escaping**: Most common issue - see "Can't Login" section above
- **Variable typos**: Ensure `SIMPLEAUTH_USERS` and `SIMPLEAUTH_SECRET` are spelled exactly
- **Format errors**: Users format is `user:hash,user2:hash2` (spaces are trimmed automatically)
- **Restart required**: After changing environment variables, restart the application
- **Hash corruption**: If authentication fails, check server logs for "invalid salt format" - indicates dollar signs weren't properly wrapped in single quotes

**Migration note:** Both `SIMPLEAUTH_USERS` and password files now use pre-hashed passwords. Use the same hash format for both. No conversion needed when switching between them.