package mail

import (
	"log"

	"github.com/abisalde/authentication-service/internal/configs"
)

var development = "princeabisal@gmail.com"

func NewMailerService(cfg *configs.Config) Mailer {

	switch cfg.Env.CurrentEnv {
	case "production":
		log.Println("INFO: Initializing Resend Mail Service for production environment.")
		return NewResendMailService(cfg.Mail.EmailAPIKey, cfg.Mail.SenderEmail)
	case "development", "test":
		log.Println("INFO: Initializing SMTP Mail Service for development/test environment.")
		return NewSMTPMailService(
			cfg.Mail.SMTPHost,
			cfg.Mail.SMTPPort,
			cfg.Mail.SMTPUsername,
			cfg.Mail.SMTPPassword,
			development,
		)
	default:
		log.Println("DEFAULT: Initializing without an environment.")
		return NewSMTPMailService(
			cfg.Mail.SMTPHost,
			cfg.Mail.SMTPPort,
			cfg.Mail.SMTPUsername,
			cfg.Mail.SMTPPassword,
			development,
		)
	}
}
