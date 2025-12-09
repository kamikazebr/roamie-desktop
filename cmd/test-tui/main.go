package main

import (
	"fmt"
	"os"

	"github.com/kamikazebr/roamie-desktop/internal/client/sshd"
)

func main() {
	fmt.Println("Testing SSHD Preflight TUI...")
	fmt.Println()

	ok, err := sshd.PromptInstall()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if ok {
		fmt.Println("\n✓ SSH check passed or user chose to continue")
	} else {
		fmt.Println("\n✗ User cancelled or SSH not available")
	}
}
