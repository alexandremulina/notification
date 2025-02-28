package services

import (
	"log/slog"
	"net/smtp"

	"go-data-distributor-notification/internal/config"
)

type EmailService struct {
	logger *slog.Logger
	config *config.Config
}

func NewEmailService(logger *slog.Logger) *EmailService {
	return &EmailService{
		logger: logger,
		config: config.GlobalConfig,
	}
}

func (s *EmailService) Send(to, subject, body string) error {
	auth := smtp.PlainAuth("",
		s.config.SMTPUsername,
		s.config.SMTPPassword,
		s.config.SMTPHost,
	)

	msg := []byte("To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	err := smtp.SendMail(
		s.config.SMTPHost+":"+s.config.SMTPPort,
		auth,
		s.config.SMTPUsername,
		[]string{to},
		msg,
	)

	if err != nil {
		s.logger.Error("Failed to send email", "error", err)
		return err
	}

	return nil
}
