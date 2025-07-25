package service

import (
	"bytes"
	"context"
	"embed"
	"html/template"
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

	return s.mailService.SendHTMLEmail(ctx, email, subject, htmlBody.String())
}
