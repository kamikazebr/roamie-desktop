# Roamie Desktop Troubleshooting Guide

This guide helps you diagnose and resolve common SSH tunnel issues using debug logging.

## Enabling Debug Mode

Debug mode adds detailed logging to help track down connection problems.

### Client Side

```bash
# Temporary (single command)
ROAMIE_DEBUG=1 roamie tunnel connect

# Persistent (current shell session)
export ROAMIE_DEBUG=1
roamie tunnel connect
```

### Server Side

```bash
# Edit systemd service configuration
sudo systemctl edit roamie-server

# Add under [Service] section:
Environment="ROAMIE_DEBUG=1"

# Save and restart
sudo systemctl restart roamie-server

# View logs
journalctl -u roamie-server -f
```

## Debug Log Format

All debug logs follow this format:
```
DEBUG: [COMPONENT] [OPERATION] message - key=value key2=value
```

**Components:**
- `[CLIENT]` - Client-side operations
- `[SERVER]` - Server-side operations
- `[API]` - API endpoint operations
- `[KEY]` - SSH key operations
- `[CONNECT]` - Connection establishment
- `[RETRY]` - Reconnection logic
- `[SSH]` - SSH protocol operations
- `[FORWARD]` - Port forwarding and data transfer
- `[KEEPALIVE]` - Keepalive packets
- `[AUTH]` - Authentication
- `[SESSION]` - Tunnel session management
- `[TUNNEL]` - Tunnel connection handling
- `[CHANNEL]` - SSH channel operations
- `[REGISTER]` - Port allocation

## Common Issues

### 1. Connection Fails Immediately

**Symptoms:**
```
Connection failed: SSH dial failed: ...
Reconnecting in 1s...
```

**Debug Pattern:**
```
DEBUG: [CLIENT] [CONNECT] Starting tunnel connection
DEBUG: [CLIENT] [SSH] Starting SSH dial - addr=server:2222
DEBUG: [CLIENT] [SSH] SSH dial failed - err=connection refused
```

**Solutions:**
- Verify server is running: `sudo systemctl status roamie-server`
- Check firewall allows port 2222
- Verify server address is correct
- Check network connectivity: `nc -zv server 2222`

### 2. Authentication Failure

**Symptoms:**
```
SSH handshake failed: ssh: handshake failed: ...
```

**Debug Pattern (Client):**
```
DEBUG: [CLIENT] [KEY] Key path resolved - path=/home/user/.config/roamie/tunnel_key
DEBUG: [CLIENT] [SSH] Starting SSH dial
DEBUG: [CLIENT] [SSH] SSH dial failed - err=ssh: handshake failed
```

**Debug Pattern (Server):**
```
DEBUG: [SERVER] [AUTH] Authentication attempt - fingerprint=SHA256:...
DEBUG: [SERVER] [AUTH] Key not found in database
```

**Solutions:**
- Register SSH key: `roamie tunnel register`
- Verify key registered: Check server logs for "SSH key registered"
- Check key fingerprint matches between client and server
- Ensure device is active and tunnel enabled

### 3. Port Already in Use

**Symptoms:**
```
Failed to listen on 0.0.0.0:10001: address already in use
```

**Debug Pattern:**
```
DEBUG: [SERVER] [SESSION] Creating listener - addr=0.0.0.0:10001
DEBUG: [SERVER] [SESSION] Listener creation failed - err=address already in use
```

**Solutions:**
- Check what's using the port: `sudo lsof -i :10001`
- Kill conflicting process or allocate different port
- Restart roamie-server service

### 4. Connection Succeeds but Data Doesn't Flow

**Symptoms:**
```
✓ Reverse tunnel established: server port 10001 → localhost:22
[No further activity]
```

**Debug Pattern:**
```
DEBUG: [CLIENT] [FORWARD] Connecting to local SSH - addr=localhost:22
DEBUG: [CLIENT] [FORWARD] Local SSH connection failed - err=connection refused
```

**Solutions:**
- Verify local SSH server is running: `sudo systemctl status ssh`
- Check SSH listening on port 22: `ss -tlnp | grep :22`
- Verify firewall allows local SSH connections

### 5. Connection Drops Periodically

**Symptoms:**
```
Keepalive failed: EOF
Connection failed: listener accept failed
```

**Debug Pattern:**
```
DEBUG: [CLIENT] [KEEPALIVE] Sending keepalive request
DEBUG: [CLIENT] [KEEPALIVE] Keepalive failed - err=EOF
DEBUG: [CLIENT] [RETRY] Connection failed - err=listener accept failed
```

**Solutions:**
- Check network stability
- Verify no aggressive firewalls dropping idle connections
- Check server logs for corresponding errors
- Consider adjusting keepalive interval (default: 10s)

### 6. Retry Loop with Exponential Backoff

**Symptoms:**
```
Reconnecting in 1s...
Reconnecting in 2s...
Reconnecting in 4s...
```

**Debug Pattern:**
```
DEBUG: [CLIENT] [RETRY] Attempting connection - delay=1s
DEBUG: [CLIENT] [RETRY] Connection failed
DEBUG: [CLIENT] [RETRY] Scheduling reconnect - delay=1s
DEBUG: [CLIENT] [RETRY] Backoff adjusted - prev=1s next=2s
```

**This is normal behavior during temporary outages. If persistent:**
- Check root cause in earlier error messages
- Verify server health: `sudo systemctl status roamie-server`
- Check server logs: `journalctl -u roamie-server -n 50`

### 7. No Tunnel Port Allocated

**Symptoms:**
```
tunnel port not found for this device
```

**Debug Pattern (API):**
```
DEBUG: [API] [REGISTER] Allocating new tunnel port - device_id=...
DEBUG: [API] [REGISTER] Port allocated - port=10001
```

**Solutions:**
- Run tunnel registration: `roamie tunnel register`
- Check device exists: `roamie auth status`
- Verify JWT token is valid

## Useful Log Patterns

### Successful Connection Flow

```
DEBUG: [CLIENT] [KEY] Existing key parsed successfully
DEBUG: [CLIENT] [CONNECT] Starting tunnel connection
DEBUG: [CLIENT] [SSH] Starting SSH dial
DEBUG: [CLIENT] [SSH] SSH dial succeeded
DEBUG: [CLIENT] [FORWARD] Listener created successfully
DEBUG: [CLIENT] [KEEPALIVE] Keepalive routine started
```

### Successful Authentication (Server)

```
DEBUG: [SERVER] [AUTH] Authentication attempt - fingerprint=SHA256:...
DEBUG: [SERVER] [AUTH] Device found - device_id=... active=true tunnel_enabled=true
DEBUG: [SERVER] [AUTH] Authentication successful - port=10001
```

### Active Data Transfer

```
DEBUG: [CLIENT] [FORWARD] Handling forward connection
DEBUG: [CLIENT] [FORWARD] Connected to local SSH
DEBUG: [CLIENT] [FORWARD] Starting bidirectional copy
DEBUG: [CLIENT] [FORWARD] Remote→Local copy complete - bytes=1234
DEBUG: [CLIENT] [FORWARD] Local→Remote copy complete - bytes=5678
```

### Tunnel Session Established (Server)

```
DEBUG: [SERVER] [SESSION] Received request - type=tcpip-forward
DEBUG: [SERVER] [SESSION] Forward request parsed - bind_addr=0.0.0.0 bind_port=10001
DEBUG: [SERVER] [SESSION] Listener created successfully
DEBUG: [SERVER] [SESSION] Starting tunnel connection handler
```

## Performance Notes

- **Debug mode disabled (default):** Negligible overhead (~single boolean check per log point)
- **Debug mode enabled:** <1% overhead for typical tunnel throughput
- Logs are written to stdout/stderr and captured by systemd/journald

## Getting Help

If issues persist after checking debug logs:

1. Capture relevant logs:
   ```bash
   # Client logs
   ROAMIE_DEBUG=1 roamie tunnel connect 2>&1 | tee client-debug.log

   # Server logs
   sudo journalctl -u roamie-server --since "5 minutes ago" > server-debug.log
   ```

2. Include system information:
   ```bash
   roamie version
   uname -a
   cat /etc/os-release
   ```

3. Report issue with logs at: https://github.com/kamikazebr/roamie-desktop/issues
