# Flutter App Migration Guide - VPN Backend Changes

## 🎯 Summary of Backend Changes

The Roamie VPN backend has been updated to **properly support device auto-registration during Firebase login**. Previously, devices were saved to the database but **never added to the WireGuard interface**, causing handshake failures.

### What Was Fixed:
1. ✅ **WireGuard peer is now properly added** during auto-registration
2. ✅ **Device replacement logic** - same device name with new keys replaces old device
3. ✅ **Idempotent registration** - same name + same key returns existing device (no error)
4. ✅ **Proper rollback** - if WireGuard fails, device is removed from database

---

## 📋 Required Flutter Changes

### **1. No Breaking Changes to API Contract**

Good news! The **API request/response format hasn't changed**. Your existing code should continue to work, but now **VPN connections will actually succeed** because devices are properly configured in WireGuard.

### **2. Device Name Strategy**

The backend now implements **replace/bypass logic** based on device name and public key:

| Scenario | Backend Behavior |
|----------|------------------|
| **New device** (name doesn't exist) | ✅ Creates new device, allocates new IP |
| **Same name + same public key** | ✅ Returns existing device (idempotent, no error) |
| **Same name + different public key** | ✅ Replaces old device, reuses same IP |

#### **Recommended Flutter Implementation:**

**Option A: Hardware-Unique Device Names** (Recommended)
```dart
Future<String> _generateDeviceName() async {
  final deviceInfo = await DeviceInfoPlugin();

  if (Platform.isAndroid) {
    final androidInfo = await deviceInfo.androidInfo;
    final uniqueId = androidInfo.id; // Android ID (unique per device)
    final username = FirebaseAuth.instance.currentUser?.email?.split('@').first ?? 'user';
    return 'android-$username-${uniqueId.substring(0, 8)}';
  } else if (Platform.isIOS) {
    final iosInfo = await deviceInfo.iosInfo;
    final uniqueId = iosInfo.identifierForVendor ?? 'unknown';
    final username = FirebaseAuth.instance.currentUser?.email?.split('@').first ?? 'user';
    return 'ios-$username-${uniqueId.substring(0, 8)}';
  }

  return 'mobile-${DateTime.now().millisecondsSinceEpoch}';
}
```

**Benefits:**
- ✅ Each physical device has unique name
- ✅ User can have multiple devices (e.g., 2 phones)
- ✅ Reinstalling app reuses same device slot (same hardware = same name)

**Option B: Simple Platform-Username Names** (Current Implementation?)
```dart
String _generateDeviceName() {
  final username = FirebaseAuth.instance.currentUser?.email?.split('@').first ?? 'user';
  return '${Platform.operatingSystem}-$username';
}
```

**Benefits:**
- ✅ Simple implementation
- ✅ Automatically replaces on reinstall/key rotation

**Drawbacks:**
- ❌ User can only have 1 device per platform
- ❌ Second phone would replace first phone's VPN access

---

### **3. Key Management**

**Current Behavior** (from logs):
```dart
// Every time _setupVPN() is called:
await _wireGuardService.generateKeyPair(); // Generates NEW keys
```

**Issue:** If you generate new keys on every login, the device name stays the same but public key changes, triggering **device replacement**.

#### **Recommended Fix:**

**Store keys persistently** and only generate once per installation:

```dart
// In your WireGuard service:
Future<void> ensureKeysExist() async {
  final prefs = await SharedPreferences.getInstance();

  // Check if keys already exist
  final existingPrivateKey = prefs.getString('wireguard_private_key');
  final existingPublicKey = prefs.getString('wireguard_public_key');

  if (existingPrivateKey != null && existingPublicKey != null) {
    print('[WireGuard] Using existing keys');
    _privateKey = existingPrivateKey;
    _publicKey = existingPublicKey;
    return;
  }

  // Generate new keys only if they don't exist
  print('[WireGuard] Generating new keys (first time)');
  await generateKeyPair();

  // Store for future use
  await prefs.setString('wireguard_private_key', _privateKey!);
  await prefs.setString('wireguard_public_key', _publicKey!);
}

// In _setupVPN():
await _wireGuardService.ensureKeysExist(); // Instead of generateKeyPair()
```

**Benefits:**
- ✅ Same device = same keys across app restarts
- ✅ Backend returns existing device (bypass logic)
- ✅ No unnecessary device replacements

---

### **4. Error Handling**

The backend may return a new field in the login response:

```json
{
  "jwt": "...",
  "user": {...},
  "device_registration_error": "WireGuard setup failed: ..." // NEW (optional)
}
```

**Update Flutter code** to handle this:

```dart
final response = await _backendAuth.loginWithFirebase(
  firebaseToken,
  deviceInfo: {
    'device_name': deviceName,
    'public_key': publicKey,
  },
);

// Check for device registration errors
if (response.containsKey('device_registration_error')) {
  final error = response['device_registration_error'];
  print('[ProfileScreen] ⚠️  Device registration warning: $error');

  // Show user-friendly message
  ScaffoldMessenger.of(context).showSnackBar(
    SnackBar(
      content: Text('VPN setup partially failed. Please contact support.'),
      backgroundColor: Colors.orange,
    ),
  );

  // Don't proceed with VPN connection if device wasn't registered
  return;
}

// Proceed with VPN setup only if device was successfully registered
if (response.containsKey('device')) {
  final device = response['device'];
  // Continue with VPN config...
}
```

---

### **5. Testing the Fix**

After updating your Flutter app with the recommendations above:

#### **Test Scenario 1: First-Time Login**
```
1. Fresh install
2. Login with Firebase
3. ✅ Device should be created
4. ✅ VPN handshake should succeed
5. ✅ Internet traffic should route through VPN
```

#### **Test Scenario 2: Re-Login (Same Keys)**
```
1. Logout from app
2. Login again with same Firebase account
3. ✅ Backend returns existing device (no error)
4. ✅ VPN handshake should succeed immediately
```

#### **Test Scenario 3: Key Rotation (Different Keys)**
```
1. Clear app data (or manually generate new keys)
2. Login with same Firebase account
3. ✅ Backend replaces old device
4. ✅ Old WireGuard peer removed
5. ✅ New peer added
6. ✅ VPN handshake should succeed
```

#### **Test Scenario 4: Multiple Devices** (if using hardware-unique names)
```
1. Login on Device A (e.g., Pixel)
2. ✅ VPN works on Device A
3. Login on Device B (e.g., Samsung) with SAME account
4. ✅ Both devices have VPN access
5. ✅ Different IPs assigned (e.g., 10.100.0.2 and 10.100.0.3)
```

---

## 🔧 Backend Admin Commands (For Debugging)

New admin commands have been added to help debug device issues:

### **List User's Devices**
```bash
./roamie-server admin list-devices --email=user@example.com
```

**Output:**
```
User: user@example.com (uuid...)
Subnet: 10.100.0.0/29
Max Devices: 5

Devices (2):
================================================================================
ID                                   Name                 VPN IP          Active
================================================================================
234de5c7-...                         android-user         10.100.0.2      Yes
7f8a9b3c-...                         ios-user             10.100.0.3      Yes
================================================================================
```

### **Delete Device by Name**
```bash
./roamie-server admin delete-device --email=user@example.com --device-name=android-user
```

This will:
1. Show device details
2. Ask for confirmation
3. Remove from WireGuard interface
4. Delete from database

**Use case:** Remove stuck/corrupted devices during testing.

---

## 🐛 Common Issues & Solutions

### **Issue 1: "Handshake did not complete after 5 seconds"**

**Cause:** Device registered in DB but not in WireGuard (this was the original bug, now fixed).

**Solution:**
- Backend fix is deployed
- Ensure Flutter is calling `/api/auth/login` with `device_info`
- Check backend logs for "✓ Peer added to WireGuard"

### **Issue 2: "device with name 'X' already exists"**

**Cause:** Old backend behavior (before this fix).

**Solution:**
- Deploy updated backend
- The error should no longer occur (replaced with bypass/replace logic)

### **Issue 3: VPN connects but no internet**

**Cause:** Firewall/NAT not configured on server.

**Solution:**
```bash
# On server, check iptables:
sudo iptables -t nat -L POSTROUTING -v

# Should show masquerade rule for 10.100.0.0/16
# If missing, backend automatically adds it on startup
```

### **Issue 4: Device keeps getting replaced**

**Cause:** Flutter generates new keys on every login.

**Solution:** Implement persistent key storage (see section 3 above).

---

## 📊 Backend Changes Summary (Technical)

For your reference, here's what changed in the backend:

### **Modified Files:**

1. **`internal/server/services/device_service.go`**
   - `RegisterDevice()` now returns `DeviceRegistrationResult` struct
   - Contains: `Device`, `ReplacedDevice`, `WasReplaced` flag
   - Implements bypass logic (same key) and replace logic (different key)

2. **`internal/server/api/device_auth.go`**
   - Added `wgManager`, `userRepo`, `deviceRepo` to handler
   - `Login()` method now:
     - Calls `wgManager.RemovePeer()` if replacing
     - Calls `wgManager.AddPeer()` for new device
     - Rolls back DB changes if WireGuard fails

3. **`internal/server/api/devices.go`**
   - Updated `RegisterDevice()` handler for replace logic
   - Same WireGuard handling as Login endpoint

4. **`cmd/server/main.go`**
   - Updated `DeviceAuthHandler` initialization with new dependencies

5. **`cmd/server/admin.go`**
   - Added `delete-device` command
   - Added `list-devices` command

---

## ✅ Migration Checklist

- [ ] Update device name generation (hardware-unique recommended)
- [ ] Implement persistent key storage
- [ ] Handle `device_registration_error` in login response
- [ ] Test first-time login
- [ ] Test re-login with same keys
- [ ] Test key rotation (app reinstall)
- [ ] Test multiple devices (if using hardware-unique names)
- [ ] Update error messages for users
- [ ] Test VPN connectivity after backend update

---

## 🤝 Support

If you encounter issues after these changes:

1. Check backend logs: `journalctl -u roamie -f`
2. List user's devices: `./roamie-server admin list-devices --email=user@example.com`
3. Check WireGuard peers: `sudo wg show`
4. Check device handshake status in Flutter logs

**Backend Logs Key Messages:**
- ✅ `✓ Peer added to WireGuard` - Device configured successfully
- ⚠️  `Warning: Failed to add peer to WireGuard` - WireGuard setup failed
- ℹ️  `Device exists with same public key, returning existing` - Bypass logic triggered
- ℹ️  `Replacing device (different public key)` - Replace logic triggered
