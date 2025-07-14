package alerts

import (
	"net/smtp"
	"os"
)

func SendEmailAlertTo(to, subject, body string) error {
	from := os.Getenv("EMAIL_USER")
	password := os.Getenv("EMAIL_PASS")

	msg := "Subject: " + subject + "\n\n" + body

	auth := smtp.PlainAuth("", from, password, "smtp.gmail.com")
	return smtp.SendMail("smtp.gmail.com:587", auth, from, []string{to}, []byte(msg))
}
