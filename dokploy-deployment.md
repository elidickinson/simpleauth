# Dokploy Deployment Guide

This guide shows how to deploy Simpleauth on Dokploy with ease. Simpleauth now supports environment variables, automatic secret generation, and flexible user management.

## Quick Start (Environment Variables)

The easiest way to deploy on Dokploy is using environment variables:

### 1. Build Local Docker Image

First build the Docker image locally:

```bash
docker build -t simpleauth:latest .
```

### 2. Create New Application

In Dokploy:
- Click "Create Application"
- Choose "Docker"
- Image: `simpleauth:latest`

### 3. Configure Environment Variables

Add these environment variables:

```bash
# Secret key (REQUIRED - generate with: openssl rand -base64 64)
SIMPLEAUTH_SECRET=your-generated-base64-secret-here

# Users (REQUIRED - format: user1:pass1,user2:pass2)
SIMPLEAUTH_USERS=admin:yourSecurePassword123,user:anotherPass

# Server (optional - defaults shown)
SIMPLEAUTH_LISTEN=:8080
SIMPLEAUTH_VERBOSE=true

# Token lifespan (optional - e.g., 168h = 7 days)
SIMPLEAUTH_LIFESPAN=168h
```

### 4. Configure Port Mapping

- Container Port: `8080`
- Host Port: Choose your desired port (e.g., `8080`)

### 5. Deploy and Test

Click "Deploy" and test by visiting your deployment URL.

## Environment Variables Reference

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `SIMPLEAUTH_LISTEN` | `:8080` | Bind address | `:8080` or `0.0.0.0:8080` |
| `SIMPLEAUTH_USERS` | (none) | Users (format: user:pass,...) | `admin:secret,user:pass` |
| `SIMPLEAUTH_SECRET` | (none) | Base64-encoded secret key | `openssl rand -base64 64` |
| `SIMPLEAUTH_LIFESPAN` | `2400h` (100 days) | Token validity period | `24h`, `7d`, `168h` |
| `SIMPLEAUTH_VERBOSE` | `false` | Enable debug logs | `true` |
| `SIMPLEAUTH_SECRET_FILE` | `/run/secrets/simpleauth.key` | Secret key file location | `/data/secret.key` |
| `SIMPLEAUTH_PASSWORD_FILE` | `/run/secrets/passwd` | Password file location | `/data/passwd` |

## Users Configuration

### Option 1: Environment Variables (Recommended for Dokploy)

```bash
SIMPLEAUTH_USERS="admin:superSecret123,user:testPass,editor:editorPass"
```

**Advantages:**
- Easy to manage in Dokploy UI
- No file mounting required
- Automatic password hashing

### Option 2: Password File (Advanced)

1. Create password file locally:
```bash
# Use the crypt tool to hash passwords
docker run --rm --entrypoint=/crypt simpleauth:latest
# This will prompt for username/password and output hash

# Create password file
echo "admin:$5$rounds=535000$..." > passwd
echo "user:$5$rounds=535000$..." >> passwd
```

2. Mount the file in Dokploy:
- Mount `./passwd` to `/run/secrets/passwd`
- Set `SIMPLEAUTH_PASSWORD_FILE` to `/run/secrets/passwd`

## Secret Management

Simpleauth requires a secret key for signing authentication tokens. You can provide it via environment variable or file.

### Option 1: Environment Variable (Recommended for Dokploy)

Generate a base64-encoded secret:

```bash
# Generate a 64-byte secret
openssl rand -base64 64
```

Then set it as an environment variable in Dokploy:

```bash
SIMPLEAUTH_SECRET="your-generated-base64-string-here"
```

### Option 2: Secret File (Advanced)

Generate a secret file:

```bash
# Generate a 64-byte secret key
dd if=/dev/urandom of=secret.key bs=1 count=64
```

Then mount `./secret.key` to `/run/secrets/simpleauth.key` (or set `SIMPLEAUTH_SECRET_FILE` to your preferred path).

## Domain Scoping (Multi-site Authentication)

If you want the auth cookie to work across multiple subdomains:

1. In your reverse proxy (Caddy/NGINX), set a header:
```
# For Caddy
header_up X-Simpleauth-Domain example.com
```

2. Set the environment variable in Dokploy:
```bash
# Simpleauth will read this from the upstream header
# No additional configuration needed
```

## Health Monitoring

Simpleauth includes a built-in health check at `/health`:

```bash
curl http://your-app:8080/health
# Response: {"status":"healthy","users":2,"secret_set":true}
```

Docker health check is automatically configured (every 30s).

## Production Best Practices

### 1. Strong Passwords

```bash
SIMPLEAUTH_USERS="admin:TrulyStrongPassword123!@,user:AnotherGoodPass456"
```

### 2. Reasonable Token Lifespan

```bash
# For production environments
SIMPLEAUTH_LIFESPAN=168h  # 7 days
# Or for higher security
SIMPLEAUTH_LIFESPAN=24h   # 1 day
```

### 3. Secret Management

Use environment variables for simplicity (recommended):
```bash
SIMPLEAUTH_SECRET="<your-base64-secret>"
```

Or if using file-based secrets, mount a volume:
- Host Path: `/var/lib/simpleauth`
- Container Path: `/data`
- Set `SIMPLEAUTH_SECRET_FILE=/data/simpleauth.key`

### 4. Monitoring

Enable verbose logging temporarily:
```bash
SIMPLEAUTH_VERBOSE=true
```

## Troubleshooting

### Can't Login

1. Check health endpoint: `GET /health`
2. Ensure `SIMPLEAUTH_USERS` is set correctly
3. Verify format: `username:password,user2:password2`

### Secret Configuration Issues

1. Ensure `SIMPLEAUTH_SECRET` environment variable is set (recommended), or
2. Ensure secret file exists at the configured path
3. If using file: verify the file has at least 64 bytes

### Health Check Shows "unhealthy"

Check the response:
- `{"status":"unhealthy","error":"no users configured"}` → Set SIMPLEAUTH_USERS
- `{"status":"unhealthy","error":"secret not properly configured"}` → Check secret file permissions

## Migration from File-based Config

If migrating from password file to environment variables:

**Before (file):**
```bash
# passwd file contents:
admin:$5$rounds=535000$...
user:$5$rounds=535000$...
```

**After (environment):**
```bash
SIMPLEAUTH_USERS="admin:plainPassword123,user:anotherPass456"
```

Simpleauth will automatically hash plain text passwords - just use the actual passwords!