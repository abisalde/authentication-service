package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
)

type SMTPMailService struct {
	smtpHost     string
	smtpPort     string
	smtpUsername string
	smtpPassword string
	senderEmail  string
}

type Mailer interface {
	SendPlainTextEmail(ctx context.Context, recipientEmail, subject, body string) error
	SendHTMLEmail(ctx context.Context, recipientEmail, subject, htmlBody string) error
}

func NewMailService(host, port, username, password, from string) *SMTPMailService {
	return &SMTPMailService{
		smtpHost:     host,
		smtpPort:     port,
		smtpUsername: username,
		smtpPassword: password,
		senderEmail:  from,
	}
}

func (s *SMTPMailService) sendEmail(recipientEmail, subject, body string) error {
	from := s.senderEmail
	to := []string{recipientEmail}

	msg := []byte(
		"From: " + from + "\r\n" +
			"To: " + recipientEmail + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"\r\n" +
			body)

	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)
	return smtp.SendMail(fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort), auth, from, to, msg)
}

func (s *SMTPMailService) SendPlainTextEmail(ctx context.Context, recipientEmail, subject, body string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:

		errChan := make(chan error, 1)
		go func() {
			errChan <- s.sendEmail(recipientEmail, subject, body)
		}()

		select {
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *SMTPMailService) SendHTMLEmail(ctx context.Context, recipientEmail, subject, htmlBody string) error {

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:

	}

	from := s.senderEmail
	to := []string{recipientEmail}

	msg := []string{
		"From: " + from,
		"To: " + recipientEmail,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/html; charset=\"utf-8\"",
		"",
		htmlBody,
	}

	message := []byte(strings.Join(msg, "\r\n"))

	auth := smtp.PlainAuth("", s.smtpUsername, s.smtpPassword, s.smtpHost)

	errChan := make(chan error, 1)
	go func() {
		errChan <- smtp.SendMail(
			fmt.Sprintf("%s:%s", s.smtpHost, s.smtpPort),
			auth,
			from,
			to,
			message,
		)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("failed to send HTML email: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("email sending canceled: %w", ctx.Err())
	}
}
