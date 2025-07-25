package mail

import (
	"context"
	"fmt"
	"log"

	"github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/resend/resend-go/v2"
)

type SMTPMailService struct {
	client      *resend.Client
	senderEmail string
}

type Mailer interface {
	SendHTMLEmail(ctx context.Context, recipientEmail, senderEmail, subject, htmlBody string) error
}

func NewMailService(emailApiKey, defaultSenderEmail string) *SMTPMailService {
	if emailApiKey == "" {
		log.Println("⚠️ WARNING: Resend API Key is empty. Email sending will likely fail.")
	}

	if defaultSenderEmail == "" {
		log.Println("WARNING: Default sender email is empty. Emails might not be sent or might be rejected.")
	}

	client := resend.NewClient(emailApiKey)
	return &SMTPMailService{
		client:      client,
		senderEmail: defaultSenderEmail,
	}
}

func (s *SMTPMailService) SendHTMLEmail(ctx context.Context, recipientEmail, subject, htmlBody string, overrideSenderEmail ...string) error {

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
			log.Printf("ERROR: Failed to send email via Resend API: %v", res.err)
			return fmt.Errorf("failed to send HTML email via Resend API: %w", res.err)
		}
		log.Printf("DEBUG: Email sent successfully via Resend. Message ID: %s", res.sent.Id)
		return nil
	case <-ctx.Done():
		log.Printf("WARNING: Email sending to %s was cancelled by context.", recipientEmail)
		return fmt.Errorf("email sending canceled: %w", ctx.Err())
	}
}
