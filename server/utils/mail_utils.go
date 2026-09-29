package utils

import (
	"os"

	"gopkg.in/gomail.v2"
	"schej.it/server/logger"
)

// Send email to the given email
func SendEmail(toEmail string, subject string, body string, contentType string) {
	if contentType == "" {
		contentType = "text/plain"
	}

	appPassword := os.Getenv("GMAIL_APP_PASSWORD")
	fromEmail := os.Getenv("SCHEJ_EMAIL_ADDRESS")
	if appPassword == "" || fromEmail == "" {
		return
	}

	m := gomail.NewMessage()
	m.SetHeader("From", fromEmail)
	m.SetHeader("To", toEmail)
	m.SetHeader("Subject", subject)
	m.SetBody(contentType, body)

	d := gomail.NewDialer("smtp.gmail.com", 587, fromEmail, appPassword)

	// Send the email to Bob, Cora and Dan.
	if err := d.DialAndSend(m); err != nil {
		logger.StdErr.Println(err)
	}
}

