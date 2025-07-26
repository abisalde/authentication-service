package service

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"strings"
)

//go:embed templates/verification_email_template.html
var emailTemplate embed.FS

func (s *AuthService) SendVerificationCodeEmail(ctx context.Context, email, code string) error {
	tmplData, err := emailTemplate.ReadFile("templates/verification_email_template.html")

	if err != nil {
		return err
	}

	tmpl, err := template.New("email").Parse(string(tmplData))
	if err != nil {
		return err
	}

	data := struct{ Code string }{Code: code}

	var htmlBody bytes.Buffer
	if err := tmpl.Execute(&htmlBody, data); err != nil {
		return err
	}
	subject := "Verify Your Email Address"
	body := fmt.Sprintf(`
		Here's your one-time passcode: %s
		
		This code will expire in 5 minutes

		Problems? Just reply to this email
	`, code)

	plainTextBody := strings.TrimSpace(body)

	return s.mailService.SendHTMLEmail(ctx, email, subject, htmlBody.String(), plainTextBody)
}
