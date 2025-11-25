package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/ssh"
	"google.golang.org/api/option"
)

func main() {
	// Load .env.production if it exists (contains Firebase credentials)
	if err := godotenv.Load(".env.production"); err != nil {
		// Try .env as fallback
		godotenv.Load(".env")
	}

	// Parse flags
	email := flag.String("email", "", "User email (document ID in Firestore)")
	key := flag.String("key", "", "SSH public key")
	name := flag.String("name", "", "Key name/label")
	flag.Parse()

	// Validate required flags
	if *email == "" || *key == "" || *name == "" {
		fmt.Println("Usage: go run scripts/add-test-ssh-key.go --email=user@example.com --key=\"ssh-ed25519 AAAA...\" --name=\"laptop\"")
		fmt.Println("\nExample:")
		fmt.Println("  go run scripts/add-test-ssh-key.go \\")
		fmt.Println("    --email=\"test@example.com\" \\")
		fmt.Println("    --key=\"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest123\" \\")
		fmt.Println("    --name=\"test-laptop\"")
		os.Exit(1)
	}

	// Validate SSH key format
	parsedKey, comment, _, _, err := ssh.ParseAuthorizedKey([]byte(*key))
	if err != nil {
		log.Fatalf("Error: Invalid SSH key format: %v", err)
	}

	// Extract key type
	keyType := parsedKey.Type()

	// Calculate fingerprint (SHA256)
	hash := sha256.Sum256(parsedKey.Marshal())
	fingerprint := "SHA256:" + base64.StdEncoding.EncodeToString(hash[:])

	// Initialize Firebase
	credentialsPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
	if credentialsPath == "" {
		log.Fatal("Error: FIREBASE_CREDENTIALS_PATH environment variable not set")
	}

	ctx := context.Background()

	// Read project ID from credentials file
	var projectID string
	credData, err := os.ReadFile(credentialsPath)
	if err != nil {
		log.Fatalf("Error reading credentials file: %v", err)
	}

	// Parse to get project_id without exposing full credentials
	var credMap map[string]interface{}
	if err := json.Unmarshal(credData, &credMap); err != nil {
		log.Fatalf("Error parsing credentials: %v", err)
	}
	if pid, ok := credMap["project_id"].(string); ok {
		projectID = pid
	} else {
		log.Fatal("Error: project_id not found in credentials file")
	}

	// Initialize Firebase with project ID and credentials
	conf := &firebase.Config{ProjectID: projectID}
	opt := option.WithCredentialsFile(credentialsPath)
	app, err := firebase.NewApp(ctx, conf, opt)
	if err != nil {
		log.Fatalf("Error initializing Firebase app: %v", err)
	}

	// Get Firestore client
	client, err := app.Firestore(ctx)
	if err != nil {
		log.Fatalf("Error getting Firestore client: %v", err)
	}
	defer client.Close()

	// Prepare data
	data := map[string]interface{}{
		"publicKey":   *key,
		"name":        *name,
		"type":        keyType,
		"fingerprint": fingerprint,
		"createdAt":   firestore.ServerTimestamp,
		"updatedAt":   firestore.ServerTimestamp,
	}

	// Add comment if present
	if comment != "" {
		data["comment"] = comment
	}

	// Add to Firestore: users/{email}/ssh_keys/{auto-id}
	docRef, _, err := client.Collection("users").
		Doc(*email).
		Collection("ssh_keys").
		Add(ctx, data)
	if err != nil {
		log.Fatalf("Error adding SSH key to Firestore: %v", err)
	}

	fmt.Println("✓ SSH key added successfully!")
	fmt.Printf("\nDetails:\n")
	fmt.Printf("  User:        %s\n", *email)
	fmt.Printf("  Document ID: %s\n", docRef.ID)
	fmt.Printf("  Name:        %s\n", *name)
	fmt.Printf("  Type:        %s\n", keyType)
	fmt.Printf("  Fingerprint: %s\n", fingerprint)
	fmt.Printf("\nFirestore path: users/%s/ssh_keys/%s\n", *email, docRef.ID)
}
