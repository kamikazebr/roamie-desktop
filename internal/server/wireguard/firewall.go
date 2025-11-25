package wireguard

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// EnsureMasqueradeRule ensures that a MASQUERADE rule exists for the given subnet
func EnsureMasqueradeRule(subnet, outInterface string) error {
	if subnet == "" {
		return fmt.Errorf("subnet cannot be empty")
	}
	if outInterface == "" {
		outInterface = "eth0" // Default to eth0
	}

	// Check if rule already exists
	checkCmd := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING",
		"-s", subnet, "-o", outInterface, "-j", "MASQUERADE")

	if err := checkCmd.Run(); err == nil {
		// Rule already exists
		log.Printf("✓ NAT rule already exists for %s", subnet)
		return nil
	}

	// Add masquerade rule
	log.Printf("Adding NAT rule for %s...", subnet)
	addCmd := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", subnet, "-o", outInterface, "-j", "MASQUERADE")

	output, err := addCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add masquerade rule: %w\nOutput: %s", err, string(output))
	}

	log.Printf("✓ NAT rule added for %s", subnet)

	// Try to save rules permanently (ignore error if netfilter-persistent not installed)
	saveCmd := exec.Command("netfilter-persistent", "save")
	if err := saveCmd.Run(); err != nil {
		log.Printf("Note: Could not save iptables rules permanently (netfilter-persistent may not be installed)")
	} else {
		log.Printf("✓ NAT rules saved permanently")
	}

	return nil
}

// EnsureForwardRule ensures that FORWARD chain allows traffic for the subnet
func EnsureForwardRule(subnet string) error {
	if subnet == "" {
		return fmt.Errorf("subnet cannot be empty")
	}

	// Check and add FORWARD ACCEPT rule for incoming traffic to subnet
	checkIn := exec.Command("iptables", "-C", "FORWARD",
		"-d", subnet, "-j", "ACCEPT")

	if err := checkIn.Run(); err != nil {
		log.Printf("Adding FORWARD rule (incoming) for %s...", subnet)
		addIn := exec.Command("iptables", "-I", "FORWARD", "1",
			"-d", subnet, "-j", "ACCEPT")
		if output, err := addIn.CombinedOutput(); err != nil {
			log.Printf("Warning: Could not add FORWARD rule (incoming): %v\nOutput: %s", err, string(output))
		}
	}

	// Check and add FORWARD ACCEPT rule for outgoing traffic from subnet
	checkOut := exec.Command("iptables", "-C", "FORWARD",
		"-s", subnet, "-j", "ACCEPT")

	if err := checkOut.Run(); err != nil {
		log.Printf("Adding FORWARD rule (outgoing) for %s...", subnet)
		addOut := exec.Command("iptables", "-I", "FORWARD", "1",
			"-s", subnet, "-j", "ACCEPT")
		if output, err := addOut.CombinedOutput(); err != nil {
			log.Printf("Warning: Could not add FORWARD rule (outgoing): %v\nOutput: %s", err, string(output))
		}
	}

	return nil
}

// GetDefaultOutInterface returns the default network interface for outgoing traffic
func GetDefaultOutInterface() string {
	// Try to detect default interface from ip route
	cmd := exec.Command("ip", "route", "show", "default")
	output, err := cmd.Output()
	if err != nil {
		return "eth0" // Fallback to eth0
	}

	// Parse output like: "default via 10.0.0.1 dev eth0"
	fields := strings.Fields(string(output))
	for i, field := range fields {
		if field == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}

	return "eth0" // Fallback
}
