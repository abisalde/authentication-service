package mail

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/smtp"

	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/resend/resend-go/v2"
)

type Mailer interface {
	SendHTMLEmail(ctx context.Context, recipientEmail, senderEmail, subject, htmlBody string, overrideSenderEmail ...string) error
}

type SMTPMailService struct {
	smtpHost     string
	smtpPort     string
	smtpUsername string
	smtpPassword string
	senderEmail  string
}

type ResendMailService struct {
	client      *resend.Client
	senderEmail string
}

func NewSMTPMailService(smtpHost, smtpPort, smtpUsername, smtpPassword, defaultSenderEmail string) *SMTPMailService {
	return &SMTPMailService{
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUsername: smtpUsername,
		smtpPassword: smtpPassword,
		senderEmail:  defaultSenderEmail,
	}
}

func NewResendMailService(apiKey, defaultSenderEmail string) *ResendMailService {
	if apiKey == "" {
		log.Println("⚠️ WARNING: Resend API Key is empty. Email sending will likely fail.")
	}
	if defaultSenderEmail == "" {
		log.Println("⚠️ WARNING:  Default sender email is empty for Resend. Emails might not be sent or might be rejected.")
	}
	client := resend.NewClient(apiKey)
	return &ResendMailService{
		client:      client,
		senderEmail: defaultSenderEmail,
	}
}

func (s *ResendMailService) SendHTMLEmail(ctx context.Context, recipientEmail, subject, htmlBody, plainTextBody string, overrideSenderEmail ...string) error {

	select {
	case <-ctx.Done():
		return errors.NewTypedError("Something went wrong, please try again", model.ErrorTypeBadRequest, map[string]interface{}{"code": "EMAIL"})
	default:

	}

	fromEmail := s.senderEmail
	if len(overrideSenderEmail) > 0 && overrideSenderEmail[0] != "" {
		fromEmail = overrideSenderEmail[0]
		log.Printf("DEBUG: Overriding sender email to: %s", fromEmail)
	} else {
		log.Printf("DEBUG: Using default sender email: %s", fromEmail)
	}

	if fromEmail == "" {
		return fmt.Errorf("sender email is empty, cannot send email")
	}

	params := &resend.SendEmailRequest{
		From:    fromEmail,
		To:      []string{recipientEmail},
		Html:    htmlBody,
		ReplyTo: "No Reply <noreply@abisalde.dev>",
		Subject: subject,
	}

	resultChan := make(chan struct {
		sent *resend.SendEmailResponse
		err  error
	}, 1)

	go func() {
		sent, err := s.client.Emails.Send(params)
		resultChan <- struct {
			sent *resend.SendEmailResponse
			err  error
		}{sent: sent, err: err}
	}()

	select {
	case res := <-resultChan:
		if res.err != nil {
			log.Printf("ERROR: ❌ Failed to send email via Resend API ❌: %v", res.err)
			return fmt.Errorf("🛠️ Failed to send HTML email via Resend API: %w", res.err)
		}
		log.Printf("DEBUG: ⚙️ Email sent successfully via Resend. Message ID: %s", res.sent.Id)
		return nil
	case <-ctx.Done():
		log.Printf("WARNING: ⚠️ Email sending to %s was cancelled by context.", recipientEmail)
		return fmt.Errorf("email sending canceled: %w", ctx.Err())
	}
}

func (s *SMTPMailService) SendHTMLEmail(ctx context.Context, recipientEmail, subject, htmlBody, plainTextBody string, overrideSenderEmail ...string) error {
	select {
	case <-ctx.Done():
		return errors.NewTypedError("Something went wrong, please try again", model.ErrorTypeBadRequest, map[string]interface{}{"code": "EMAIL"})
	default:

	}

	fromEmail := s.senderEmail
	if len(overrideSenderEmail) > 0 && overrideSenderEmail[0] != "" {
		fromEmail = overrideSenderEmail[0]
		log.Printf("DEBUG: Overriding sender email to: %s", fromEmail)
	} else {
		log.Printf("DEBUG: Using default sender email: %s", fromEmail)
	}

	if fromEmail == "" {
		return fmt.Errorf("sender email is empty, cannot send email")
	}

	var buf bytes.Buffer

	headers := map[string]string{
		"From":         fromEmail,
		"To":           recipientEmail,
		"Subject":      subject,
		"MIME-Version": "1.0",
		"Content-Type": "multipart/alternative; boundary=\"MIMEBOUNDARY\"",
	}

	for k, v := range headers {
		buf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	buf.WriteString("\r\n")

	buf.WriteString("--MIMEBOUNDARY\r\n")
	buf.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(plainTextBody)
	buf.WriteString("\r\n")

	buf.WriteString("--MIMEBOUNDARY\r\n")
	buf.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(htmlBody)
	buf.WriteString("\r\n")

	buf.WriteString("--MIMEBOUNDARY--\r\n")

	message := buf.Bytes()
	to := []string{recipientEmail}
	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)

	errChan := make(chan error, 1)
	go func() {
		errChan <- smtp.SendMail(
			fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort),
			auth,
			fromEmail,
			to,
			message,
		)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			log.Printf("ERROR: SMTP: Failed to send email: %v", err)
			return fmt.Errorf("SMTP: failed to send HTML email with plain text fallback: %w", err)
		}
		log.Printf("INFO: SMTP: Email sent successfully to %s", recipientEmail)
		return nil
	case <-ctx.Done():
		log.Printf("WARNING: SMTP: Email sending to %s was cancelled by context.", recipientEmail)
		return fmt.Errorf("SMTP: email sending canceled: %w", ctx.Err())
	}

}
