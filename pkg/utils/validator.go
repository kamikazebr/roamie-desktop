package utils

import (
	"net"
	"regexp"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func IsValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

func IsValidCIDR(cidr string) bool {
	_, _, err := net.ParseCIDR(cidr)
	return err == nil
}

func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func IsValidWireGuardKey(key string) bool {
	// WireGuard keys are 44 base64 characters
	if len(key) != 44 {
		return false
	}
	matched, _ := regexp.MatchString(`^[A-Za-z0-9+/]{43}=$`, key)
	return matched
}
