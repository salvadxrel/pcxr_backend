package email

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"

	"gopkg.in/gomail.v2"
)

type SMTPSender struct {
	host     string
	port     int
	user     string
	password string
}

//go:embed templates/reset_password.html
var templateFS embed.FS

func NewSMTPSender(host string, port int, user string, password string) *SMTPSender {
	return &SMTPSender{host, port, user, password}
}

func (s *SMTPSender) SendPasswordRest(to, resetLink string) error {
	tmpl, err := template.ParseFS(templateFS, "templates/reset_password.html")
	if err != nil {
		return fmt.Errorf("parse email template: %w", err)
	}
	var body bytes.Buffer
	if err := tmpl.Execute(&body, struct{ ResetLink string }{ResetLink: resetLink}); err != nil {
		return fmt.Errorf("execute email template: %w", err)
	}
	msg := gomail.NewMessage()
	msg.SetHeader("From", s.user)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", "Восстановление пароля — PCXR")
	msg.SetBody("text/html", body.String())

	dailer := gomail.NewDialer(s.host, s.port, s.user, s.password)
	return dailer.DialAndSend(msg)
}
