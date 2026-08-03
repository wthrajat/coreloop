package alerts

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type Service struct{ recipient, host, port, username, password, from string }

func FromEnv(recipient string) *Service {
	return &Service{recipient: recipient, host: os.Getenv("SMTP_HOST"), port: envOr("SMTP_PORT", "587"), username: os.Getenv("SMTP_USERNAME"), password: os.Getenv("SMTP_PASSWORD"), from: envOr("SMTP_FROM", recipient)}
}

func (service *Service) Send(ctx context.Context, subject, body string) error {
	if service.recipient == "" {
		slog.WarnContext(ctx, "admin alert has no recipient", "subject", subject)
		return nil
	}
	if service.host == "" || service.username == "" || service.password == "" {
		slog.ErrorContext(ctx, "admin email could not be sent because SMTP is not configured", "subject", subject, "recipient", service.recipient)
		return fmt.Errorf("SMTP is not configured")
	}
	address := net.JoinHostPort(service.host, service.port)
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	connection, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer connection.Close()
	client, err := smtp.NewClient(connection, service.host)
	if err != nil {
		return err
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: service.host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if err := client.Auth(smtp.PlainAuth("", service.username, service.password, service.host)); err != nil {
		return err
	}
	if err := client.Mail(service.from); err != nil {
		return err
	}
	if err := client.Rcpt(service.recipient); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	message := "To: " + service.recipient + "\r\nFrom: " + service.from + "\r\nSubject: " + sanitize(subject) + "\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n" + body
	if _, err := writer.Write([]byte(message)); err != nil {
		return err
	}
	return writer.Close()
}
func sanitize(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")
}
func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
