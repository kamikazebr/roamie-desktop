#!/usr/bin/env python3
"""
Roamie Biometric Sudo PAM Script

This script is called by PAM when a user attempts to use sudo or other privileged operations.
It sends an authentication request to the Culodi VPN server, which forwards it to the user's
mobile device. The user can then approve or deny the request using biometric authentication.

Installation:
1. Copy this script to /usr/local/bin/roamie_biometric_auth.py
2. Make it executable: chmod +x /usr/local/bin/roamie_biometric_auth.py
3. Configure PAM in /etc/pam.d/sudo-biometric

Usage:
    roamie_biometric_auth.py [command]

Returns:
    0: Authentication approved
    1: Authentication denied or error
"""

import os
import sys
import json
import time
import socket
import requests
from typing import Dict, Optional, Tuple

# Configuration
API_BASE_URL = "http://10.100.0.1:8080"
JWT_TOKEN_PATH = "/root/.roamie_jwt"
REQUEST_TIMEOUT = 30  # seconds
POLL_INTERVAL = 2  # seconds

def log(message: str):
    """Log message to syslog for debugging"""
    # In production, this should use syslog
    # For now, write to stderr (PAM captures stderr)
    print(f"[RoamieBioSudo] {message}", file=sys.stderr)

def read_jwt_token() -> Optional[str]:
    """Read JWT token from secure file"""
    try:
        if not os.path.exists(JWT_TOKEN_PATH):
            log(f"Error: JWT token file not found at {JWT_TOKEN_PATH}")
            return None

        # Check file permissions (should be 0600 or 0400)
        stat_info = os.stat(JWT_TOKEN_PATH)
        if stat_info.st_mode & 0o077:
            log(f"Warning: JWT token file has insecure permissions")

        with open(JWT_TOKEN_PATH, 'r') as f:
            token = f.read().strip()
            if not token:
                log("Error: JWT token file is empty")
                return None
            return token
    except Exception as e:
        log(f"Error reading JWT token: {e}")
        return None

def get_hostname() -> str:
    """Get system hostname"""
    try:
        return socket.gethostname()
    except Exception:
        return "unknown"

def create_auth_request(token: str, command: str) -> Optional[str]:
    """Create a new biometric authentication request"""
    url = f"{API_BASE_URL}/api/auth/biometric/request"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

    username = os.getenv("USER", "unknown")
    hostname = get_hostname()

    data = {
        "username": username,
        "hostname": hostname,
        "command": command
    }

    try:
        log(f"Creating auth request for {username}@{hostname}")
        response = requests.post(url, headers=headers, json=data, timeout=5)

        if response.status_code == 201:
            result = response.json()
            request_id = result.get("request_id")
            log(f"Auth request created: {request_id}")
            return request_id
        else:
            log(f"Failed to create auth request: {response.status_code} - {response.text}")
            return None
    except requests.exceptions.RequestException as e:
        log(f"Network error creating auth request: {e}")
        return None
    except Exception as e:
        log(f"Error creating auth request: {e}")
        return None

def poll_auth_status(token: str, request_id: str) -> Tuple[bool, str]:
    """
    Poll the auth request status
    Returns: (success: bool, status: str)
    """
    url = f"{API_BASE_URL}/api/auth/biometric/poll/{request_id}"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

    try:
        response = requests.get(url, headers=headers, timeout=5)

        if response.status_code == 200:
            result = response.json()
            status = result.get("status", "unknown")
            return True, status
        else:
            log(f"Failed to poll auth status: {response.status_code}")
            return False, "error"
    except requests.exceptions.RequestException as e:
        log(f"Network error polling auth status: {e}")
        return False, "error"
    except Exception as e:
        log(f"Error polling auth status: {e}")
        return False, "error"

def wait_for_auth_response(token: str, request_id: str) -> bool:
    """
    Wait for user to respond to auth request
    Returns: True if approved, False if denied/expired/error
    """
    start_time = time.time()

    log("Waiting for user response...")

    while True:
        elapsed = time.time() - start_time

        if elapsed > REQUEST_TIMEOUT:
            log("Timeout waiting for user response")
            return False

        success, status = poll_auth_status(token, request_id)

        if not success:
            log("Error polling auth status")
            return False

        if status == "approved":
            log("Authentication APPROVED by user")
            return True
        elif status == "denied":
            log("Authentication DENIED by user")
            return False
        elif status in ["expired", "timeout"]:
            log(f"Authentication request {status}")
            return False
        elif status == "pending":
            # Still waiting, continue polling
            time.sleep(POLL_INTERVAL)
        else:
            log(f"Unknown status: {status}")
            return False

def main():
    """Main entry point"""
    # Get command from arguments or environment
    command = " ".join(sys.argv[1:]) if len(sys.argv) > 1 else os.getenv("PAM_RHOST", "sudo")

    log(f"Biometric authentication requested for: {command}")

    # Read JWT token
    token = read_jwt_token()
    if not token:
        log("Failed to read JWT token")
        return 1

    # Create auth request
    request_id = create_auth_request(token, command)
    if not request_id:
        log("Failed to create auth request")
        return 1

    # Wait for response
    approved = wait_for_auth_response(token, request_id)

    if approved:
        log("Authentication SUCCESSFUL")
        return 0
    else:
        log("Authentication FAILED")
        return 1

if __name__ == "__main__":
    try:
        sys.exit(main())
    except KeyboardInterrupt:
        log("Authentication cancelled by user")
        sys.exit(1)
    except Exception as e:
        log(f"Unexpected error: {e}")
        sys.exit(1)
