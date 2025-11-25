package services

import (
	"fmt"
	"os"

	"github.com/resendlabs/resend-go"
)

type EmailService struct {
	client    *resend.Client
	fromEmail string
}

func NewEmailService() (*EmailService, error) {
	apiKey := os.Getenv("RESEND_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("RESEND_API_KEY environment variable not set")
	}

	fromEmail := os.Getenv("FROM_EMAIL")
	if fromEmail == "" {
		fromEmail = "noreply@roamie.com"
	}

	client := resend.NewClient(apiKey)

	return &EmailService{
		client:    client,
		fromEmail: fromEmail,
	}, nil
}

func (s *EmailService) SendAuthCode(email, code string) error {
	// Skip email sending in test mode
	if os.Getenv("SKIP_EMAIL_SEND") == "true" {
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{email},
		Subject: "Your Roamie VPN Authentication Code",
		Html: fmt.Sprintf(`
			<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
				<h2 style="color: #333;">Roamie VPN Login Code</h2>
				<p>Your authentication code is:</p>
				<div style="background-color: #f4f4f4; padding: 20px; text-align: center; font-size: 32px; font-weight: bold; letter-spacing: 8px; margin: 20px 0;">
					%s
				</div>
				<p style="color: #666;">This code will expire in 5 minutes.</p>
				<p style="color: #666;">If you didn't request this code, please ignore this email.</p>
				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
				<p style="color: #999; font-size: 12px;">Roamie VPN - Secure Multi-Device WireGuard Network</p>
			</div>
		`, code),
	}

	_, err := s.client.Emails.Send(params)
	return err
}

func (s *EmailService) SendWelcomeEmail(email string, subnet string) error {
	// Skip email sending in test mode
	if os.Getenv("SKIP_EMAIL_SEND") == "true" {
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.fromEmail,
		To:      []string{email},
		Subject: "Welcome to Roamie VPN",
		Html: fmt.Sprintf(`
			<div style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
				<h2 style="color: #333;">Welcome to Roamie VPN!</h2>
				<p>Your VPN account has been successfully created.</p>
				<p><strong>Your dedicated subnet:</strong> <code>%s</code></p>
				<p>All your devices will be assigned IPs within this subnet, allowing them to communicate securely with each other.</p>
				<h3>Next Steps:</h3>
				<ol>
					<li>Install the Roamie VPN client</li>
					<li>Run <code>roamie device add</code> to register your first device</li>
					<li>Connect and enjoy secure networking!</li>
				</ol>
				<p style="color: #666;">You can add up to 5 devices to your account.</p>
				<hr style="border: none; border-top: 1px solid #eee; margin: 30px 0;">
				<p style="color: #999; font-size: 12px;">Roamie VPN - Secure Multi-Device WireGuard Network</p>
			</div>
		`, subnet),
	}

	_, err := s.client.Emails.Send(params)
	return err
}
