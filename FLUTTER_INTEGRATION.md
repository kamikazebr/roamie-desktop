# Flutter App - SSH Tunnel Integration Guide

This document provides guidance for integrating SSH reverse tunnel functionality into the Roamie Flutter app.

## Overview

The SSH reverse tunnel feature allows users to SSH into their devices remotely through the Roamie server, even when devices are behind NAT or firewalls. This works as an alternative/complement to the WireGuard VPN.

### Architecture

```
Flutter App (Mobile)
        ↓ (Enable/Disable tunnel per device)
    Roamie Server
        ↑ (SSH tunnel on port 2222)
   Device (Linux/Mac/Windows)
```

**Flow:**
1. Device runs `roamie tunnel register` to allocate a port (10000-20000)
2. Flutter app enables tunnel for specific device
3. Device establishes reverse SSH tunnel to server
4. Flutter app can now SSH into device via: `ssh user@server -p <allocated_port>`

## API Endpoints

All endpoints require JWT authentication via `Authorization: Bearer <token>` header.

### 1. Get Tunnel Status

**Endpoint:** `GET /api/tunnel/status`

**Response:**
```json
{
  "tunnels": [
    {
      "device_id": "uuid",
      "device_name": "android-username-a1b2c3d4",
      "port": 10001,
      "enabled": true,
      "connected": false
    }
  ],
  "server_host": "server.example.com"
}
```

**Flutter Implementation:**
```dart
Future<TunnelStatusResponse> getTunnelStatus() async {
  final response = await http.get(
    Uri.parse('$serverUrl/api/tunnel/status'),
    headers: {
      'Authorization': 'Bearer $jwt',
    },
  );

  if (response.statusCode == 200) {
    return TunnelStatusResponse.fromJson(jsonDecode(response.body));
  } else {
    throw Exception('Failed to get tunnel status');
  }
}
```

### 2. Enable Tunnel for Device

**Endpoint:** `PATCH /api/devices/{device_id}/tunnel/enable`

**Response:**
```json
{
  "message": "tunnel enabled"
}
```

**Flutter Implementation:**
```dart
Future<void> enableTunnel(String deviceId) async {
  final response = await http.patch(
    Uri.parse('$serverUrl/api/devices/$deviceId/tunnel/enable'),
    headers: {
      'Authorization': 'Bearer $jwt',
    },
  );

  if (response.statusCode != 200) {
    throw Exception('Failed to enable tunnel');
  }
}
```

### 3. Disable Tunnel for Device

**Endpoint:** `PATCH /api/devices/{device_id}/tunnel/disable`

**Response:**
```json
{
  "message": "tunnel disabled"
}
```

**Flutter Implementation:**
```dart
Future<void> disableTunnel(String deviceId) async {
  final response = await http.patch(
    Uri.parse('$serverUrl/api/devices/$deviceId/tunnel/disable'),
    headers: {
      'Authorization': 'Bearer $jwt',
    },
  );

  if (response.statusCode != 200) {
    throw Exception('Failed to disable tunnel');
  }
}
```

### 4. List Devices (Extended with Tunnel Info)

**Endpoint:** `GET /api/devices`

**Response:**
```json
{
  "devices": [
    {
      "id": "uuid",
      "device_name": "android-username-a1b2c3d4",
      "hardware_id": "a1b2c3d4",
      "os_type": "android",
      "vpn_ip": "10.100.0.2",
      "tunnel_port": 10001,
      "tunnel_enabled": true,
      "is_online": true,
      "last_seen": "2025-01-12T10:30:00Z",
      "created_at": "2025-01-10T08:00:00Z"
    }
  ]
}
```

## Data Models

### TunnelStatusResponse
```dart
class TunnelStatusResponse {
  final List<TunnelInfo> tunnels;
  final String serverHost;

  TunnelStatusResponse({
    required this.tunnels,
    required this.serverHost,
  });

  factory TunnelStatusResponse.fromJson(Map<String, dynamic> json) {
    return TunnelStatusResponse(
      tunnels: (json['tunnels'] as List)
          .map((t) => TunnelInfo.fromJson(t))
          .toList(),
      serverHost: json['server_host'] ?? '',
    );
  }
}
```

### TunnelInfo
```dart
class TunnelInfo {
  final String deviceId;
  final String deviceName;
  final int port;
  final bool enabled;
  final bool connected;

  TunnelInfo({
    required this.deviceId,
    required this.deviceName,
    required this.port,
    required this.enabled,
    required this.connected,
  });

  factory TunnelInfo.fromJson(Map<String, dynamic> json) {
    return TunnelInfo(
      deviceId: json['device_id'] ?? '',
      deviceName: json['device_name'] ?? '',
      port: json['port'] ?? 0,
      enabled: json['enabled'] ?? false,
      connected: json['connected'] ?? false,
    );
  }
}
```

### Device Model Extension
```dart
class Device {
  // ... existing fields

  final int? tunnelPort;
  final bool tunnelEnabled;

  // Add to fromJson:
  tunnelPort: json['tunnel_port'],
  tunnelEnabled: json['tunnel_enabled'] ?? false,
}
```

## UI Implementation

### 1. Device List Screen

Add tunnel toggle for each device:

```dart
class DeviceTile extends StatelessWidget {
  final Device device;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      title: Text(device.deviceName),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('VPN IP: ${device.vpnIp}'),
          if (device.tunnelPort != null)
            Text('Tunnel Port: ${device.tunnelPort}'),
          if (device.isOnline)
            Text('● Online', style: TextStyle(color: Colors.green))
          else
            Text('○ Offline', style: TextStyle(color: Colors.grey)),
        ],
      ),
      trailing: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          // Tunnel toggle
          if (device.tunnelPort != null)
            Switch(
              value: device.tunnelEnabled,
              onChanged: (enabled) async {
                if (enabled) {
                  await apiClient.enableTunnel(device.id);
                } else {
                  await apiClient.disableTunnel(device.id);
                }
                // Refresh device list
              },
            ),
          // SSH button (only if tunnel enabled and online)
          if (device.tunnelEnabled && device.isOnline)
            IconButton(
              icon: Icon(Icons.terminal),
              onPressed: () => _openSSH(device),
            ),
        ],
      ),
    );
  }

  void _openSSH(Device device) {
    // Show SSH connection details
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('SSH Connection'),
        content: SelectableText(
          'ssh username@server.example.com -p ${device.tunnelPort}',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: Text('Close'),
          ),
        ],
      ),
    );
  }
}
```

### 2. Post-Registration Flow

After device registration, show connection choice:

```dart
Future<void> handleDeviceRegistration() async {
  // After successful registration
  final choice = await showDialog<String>(
    context: context,
    builder: (context) => AlertDialog(
      title: Text('Choose Connection Method'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text('How would you like to connect to this device?'),
          SizedBox(height: 16),
          ListTile(
            leading: Icon(Icons.vpn_key),
            title: Text('SSH Tunnel (Recommended)'),
            subtitle: Text('Remote access via SSH'),
            onTap: () => Navigator.pop(context, 'tunnel'),
          ),
          ListTile(
            leading: Icon(Icons.security),
            title: Text('WireGuard VPN'),
            subtitle: Text('Full network access'),
            onTap: () => Navigator.pop(context, 'vpn'),
          ),
        ],
      ),
    ),
  );

  if (choice == 'tunnel') {
    // Enable tunnel for device
    await apiClient.enableTunnel(deviceId);

    // Show instructions
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('Tunnel Enabled'),
        content: Text(
          'The device will establish an SSH tunnel.\n\n'
          'On the device, run:\n'
          '  roamie tunnel register\n'
          '  roamie tunnel start'
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: Text('OK'),
          ),
        ],
      ),
    );
  } else if (choice == 'vpn') {
    // Existing VPN setup flow
  }
}
```

### 3. Tunnel Status Badge

Show tunnel status on device tiles:

```dart
Widget buildTunnelBadge(Device device) {
  if (device.tunnelPort == null) {
    return SizedBox.shrink(); // No tunnel registered
  }

  if (!device.tunnelEnabled) {
    return Chip(
      label: Text('Tunnel Disabled'),
      backgroundColor: Colors.grey,
      avatar: Icon(Icons.cloud_off, size: 16),
    );
  }

  if (device.isOnline) {
    return Chip(
      label: Text('Tunnel Ready'),
      backgroundColor: Colors.green,
      avatar: Icon(Icons.cloud_done, size: 16),
    );
  } else {
    return Chip(
      label: Text('Device Offline'),
      backgroundColor: Colors.orange,
      avatar: Icon(Icons.cloud_off, size: 16),
    );
  }
}
```

## Error Handling

Common error responses:

### 404 - Device Not Found
```json
{
  "error": "device not found"
}
```

### 401 - Unauthorized
```json
{
  "error": "unauthorized"
}
```

### 500 - Internal Server Error
```json
{
  "error": "failed to enable tunnel"
}
```

**Flutter Error Handling:**
```dart
Future<void> enableTunnelWithErrorHandling(String deviceId) async {
  try {
    await apiClient.enableTunnel(deviceId);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('Tunnel enabled successfully')),
    );
  } on UnauthorizedException {
    // JWT expired, redirect to login
    Navigator.pushReplacementNamed(context, '/login');
  } on NotFoundException {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Device not found'),
        backgroundColor: Colors.red,
      ),
    );
  } catch (e) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Failed to enable tunnel: $e'),
        backgroundColor: Colors.red,
      ),
    );
  }
}
```

## Polling for Updates

To get real-time tunnel status, implement polling:

```dart
class DeviceListProvider extends ChangeNotifier {
  Timer? _pollTimer;

  void startPolling() {
    _pollTimer = Timer.periodic(Duration(seconds: 10), (_) async {
      await refreshDevices();
    });
  }

  void stopPolling() {
    _pollTimer?.cancel();
  }

  Future<void> refreshDevices() async {
    try {
      final devices = await apiClient.getDevices();
      // Update state
      notifyListeners();
    } catch (e) {
      // Handle error
    }
  }

  @override
  void dispose() {
    stopPolling();
    super.dispose();
  }
}
```

## Testing

### Manual Testing Steps

1. **Enable Tunnel**:
   - Open Flutter app
   - Navigate to devices list
   - Toggle tunnel switch ON for a device
   - Verify API call succeeds

2. **Device Side**:
   - On the device, run: `roamie tunnel status`
   - Should show "Tunnel is enabled"
   - Run: `roamie tunnel start`
   - Should establish connection

3. **SSH Connection**:
   - From Flutter app or any SSH client:
   - `ssh username@server.example.com -p <port>`
   - Should connect to device

4. **Disable Tunnel**:
   - Toggle switch OFF in Flutter app
   - Device tunnel should disconnect
   - Verify no SSH connections possible

### Unit Tests

```dart
void main() {
  group('TunnelApi', () {
    test('enableTunnel returns success', () async {
      // Mock HTTP client
      final api = TunnelApi(mockHttpClient);

      when(mockHttpClient.patch(any, headers: anyNamed('headers')))
          .thenAnswer((_) async => http.Response('{"message":"tunnel enabled"}', 200));

      await api.enableTunnel('device-id-123');

      verify(mockHttpClient.patch(
        Uri.parse('$serverUrl/api/devices/device-id-123/tunnel/enable'),
        headers: {'Authorization': 'Bearer mock-jwt'},
      )).called(1);
    });
  });
}
```

## Security Considerations

1. **JWT Expiration**: Always check JWT validity before API calls
2. **HTTPS Only**: Never use HTTP for production
3. **Port Exposure**: Ports 10000-20000 are exposed - ensure server firewall configured
4. **SSH Keys**: Users should use SSH key authentication, not passwords
5. **Device Verification**: Always verify device belongs to authenticated user

## Troubleshooting

### "Failed to enable tunnel"
- Check JWT token is valid
- Verify device ID is correct
- Check device belongs to authenticated user

### "No tunnel port allocated"
- Device needs to run `roamie tunnel register` first
- Check server logs for allocation errors

### "Device offline"
- Device daemon not running
- Run `roamie daemon` on device to maintain heartbeat

### "Connection refused" when SSH
- Check tunnel is enabled in Flutter app
- Verify device is running `roamie tunnel start`
- Check server SSH tunnel server is running (port 2222)

## Migration Notes

### Backward Compatibility

All existing API endpoints remain unchanged. Tunnel fields are optional:
- `tunnel_port`: `null` if not registered
- `tunnel_enabled`: defaults to `false`
- `is_online`: calculated from heartbeat (existing feature)

### Database Schema

New fields added in migration `011_tunnel_keys.sql`:
- `tunnel_ssh_key TEXT`
- `tunnel_enabled BOOLEAN DEFAULT false`

No breaking changes to existing fields.

## Next Steps for Flutter Team

1. **Immediate** (MVP):
   - Add tunnel toggle to device list
   - Implement enable/disable API calls
   - Show tunnel port in device details

2. **Short-term** (Enhanced UX):
   - Post-registration connection choice dialog
   - SSH connection details modal
   - Tunnel status badges

3. **Long-term** (Advanced Features):
   - In-app SSH terminal (using flutter_libssh or similar)
   - Tunnel connection logs
   - Port forwarding management

## Support

For questions or issues:
- Check server logs: `journalctl -u roamie-server -f`
- Check device logs: `journalctl -u roamie-daemon -f`
- API documentation: `https://server.example.com/api/docs`
