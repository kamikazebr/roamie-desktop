# Roamie VPN Server - Coolify Deployment

This guide explains how to deploy Roamie VPN Server on Coolify.

## Prerequisites

- Coolify instance with Docker support
- Server with kernel WireGuard module loaded (`modprobe wireguard`)
- Domain configured in Coolify for the API endpoint
- Ports available: 51820/UDP (WireGuard), 2222 (SSH Tunnel)

## Quick Start

### 1. Create a New Service in Coolify

1. Go to your Coolify dashboard
2. Create a new **Docker Compose** service
3. Connect your Git repository or upload files

### 2. Configure Environment Variables

In Coolify's environment variables UI, add all variables from `.env.coolify.example`:

**Required variables:**
- `POSTGRES_PASSWORD` - Strong database password
- `JWT_SECRET` - Generate with `openssl rand -base64 32`
- `RESEND_API_KEY` - From [resend.com](https://resend.com)
- `WG_SERVER_PUBLIC_ENDPOINT` - Your server's public IP:51820

### 3. Configure Ports

In Coolify's port configuration:

| Port | Protocol | Purpose | Proxy |
|------|----------|---------|-------|
| 8080 | TCP | HTTP API | Yes (Coolify handles SSL) |
| 51820 | UDP | WireGuard | No (direct exposure) |
| 2222 | TCP | SSH Tunnel | No (direct exposure) |

### 4. Configure Domain

1. Add your domain in Coolify (e.g., `vpn-api.yourdomain.com`)
2. Coolify will automatically provision SSL via Let's Encrypt
3. Point the domain to port 8080

### 5. Deploy

Click Deploy in Coolify. The service will:
1. Build the Docker image
2. Start PostgreSQL and wait for health check
3. Run database migrations automatically
4. Start the VPN server

## Architecture

```
Internet
    │
    ├── HTTPS (:443) ──► Coolify Proxy ──► roamie-server:8080
    │
    ├── UDP (:51820) ──────────────────► roamie-server:51820 (WireGuard)
    │
    └── TCP (:2222) ───────────────────► roamie-server:2222 (SSH Tunnel)
```

## Firewall Configuration

Ensure your server firewall allows:

```bash
# UFW example
sudo ufw allow 51820/udp  # WireGuard
sudo ufw allow 2222/tcp   # SSH Tunnel
# Port 443 should already be open for Coolify
```

## Health Check

The server exposes a health endpoint:

```bash
curl https://vpn-api.yourdomain.com/health
```

## Logs

View logs in Coolify UI or:

```bash
# Via Coolify CLI or Docker
docker logs roamie-server -f
```

## Migrating to Supabase

When ready to migrate from internal PostgreSQL to Supabase:

1. Export data from internal PostgreSQL (if needed)
2. Create a Supabase project
3. Update `DATABASE_URL` in Coolify to Supabase connection string:
   ```
   DATABASE_URL=postgres://postgres:[PASSWORD]@db.[PROJECT].supabase.co:5432/postgres
   ```
4. Run migrations on Supabase (server does this automatically on startup)
5. Remove the `postgres` service from docker-compose if desired

## Troubleshooting

### WireGuard not working

1. Check kernel module: `lsmod | grep wireguard`
2. If not loaded: `sudo modprobe wireguard`
3. Verify container has NET_ADMIN capability

### Database connection issues

1. Check PostgreSQL health: `docker logs roamie-postgres`
2. Verify `DATABASE_URL` format
3. Ensure postgres service started before roamie-server

### SSL/Domain issues

1. Check Coolify proxy logs
2. Verify domain DNS points to server
3. Wait for Let's Encrypt certificate provisioning

## Running Multiple Environments (Production + Dev)

You can run both production and development environments on the same server without conflicts.

### Files

| Environment | Compose File | Env Example |
|-------------|--------------|-------------|
| Production | `docker-compose.coolify.yml` | `.env.coolify.example` |
| Development | `docker-compose.coolify-dev.yml` | `.env.coolify-dev.example` |

### Key Differences

| Resource | Production | Development |
|----------|------------|-------------|
| Container names | `roamie-*` | `roamie-dev-*` |
| WireGuard port | 51820/udp | 51821/udp |
| SSH Tunnel port | 2222 | 2223 |
| WireGuard interface | `wg0` | `wg-dev` |
| VPN network | 10.100.0.0/16 | 10.101.0.0/16 |
| Tunnel port range | 10000-20000 | 20000-30000 |
| Database | `roamie_vpn` | `roamie_vpn_dev` |
| Volumes | `roamie_*` | `roamie_dev_*` |

### Firewall for Both Environments

```bash
# Production
sudo ufw allow 51820/udp  # WireGuard prod
sudo ufw allow 2222/tcp   # SSH Tunnel prod

# Development
sudo ufw allow 51821/udp  # WireGuard dev
sudo ufw allow 2223/tcp   # SSH Tunnel dev
```

### Deploying Both in Coolify

1. Create two separate services in Coolify
2. **Production**: Use `docker-compose.coolify.yml`
3. **Development**: Use `docker-compose.coolify-dev.yml`
4. Configure different domains (e.g., `vpn-api.domain.com` and `vpn-api-dev.domain.com`)
5. Set environment variables from respective `.env.*.example` files

## Security Notes

- Never commit `.env` files or actual credentials
- Use Coolify's secrets management for sensitive values
- Regularly rotate `JWT_SECRET` (will invalidate all tokens)
- Keep `POSTGRES_PASSWORD` secure and unique
- Use different `JWT_SECRET` for production and development
