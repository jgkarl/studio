// Package mail sends transactional email — real SMTP (net/smtp, stdlib only) when SMTP_HOST is
// configured, or a console transport that just logs the message when it isn't. Same pattern as
// Rails' letter_opener / Django's console email backend: fully testable without real mail
// infrastructure. Mirrors the original app's lib/mail/mailer.ts.
package mail

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Config struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

type Mailer struct {
	cfg Config
}

func New(cfg Config) *Mailer {
	return &Mailer{cfg: cfg}
}

func (m *Mailer) Send(msg Message) error {
	if m.cfg.Host == "" {
		sendViaConsole(msg)
		return nil
	}

	from := m.cfg.From
	if from == "" {
		from = "Studio <no-reply@localhost>"
	}
	body := buildMIME(fromAddress(from), msg)
	addr := m.cfg.Host + ":" + m.cfg.Port

	var auth smtp.Auth
	if m.cfg.User != "" {
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)
	}

	if m.cfg.Port == "465" {
		return sendImplicitTLS(addr, m.cfg.Host, auth, fromAddress(from), msg.To, body)
	}
	// net/smtp.SendMail upgrades to STARTTLS automatically when the server advertises it —
	// correct for the usual submission port 587 (and plain 25).
	return smtp.SendMail(addr, auth, fromAddress(from), []string{msg.To}, body)
}

// fromAddress extracts a bare address from a "Name <addr>" or plain "addr" string, for the
// MAIL FROM envelope command (which rejects display names).
func fromAddress(from string) string {
	if start := strings.Index(from, "<"); start >= 0 {
		if end := strings.Index(from[start:], ">"); end >= 0 {
			return from[start+1 : start+end]
		}
	}
	return from
}

func sendImplicitTLS(addr, host string, auth smtp.Auth, from, to string, body []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

const mimeBoundary = "studio-mail-boundary"

// buildMIME hand-builds a minimal multipart/alternative (text + html) RFC 822 message —
// deliberately not pulling in an email-building library for two short transactional templates.
func buildMIME(from string, msg Message) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")

	if msg.HTML == "" {
		b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
		b.WriteString(msg.Text)
		return []byte(b.String())
	}

	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", mimeBoundary)
	fmt.Fprintf(&b, "--%s\r\n", mimeBoundary)
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
	b.WriteString(msg.Text)
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s\r\n", mimeBoundary)
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n\r\n")
	b.WriteString(msg.HTML)
	b.WriteString("\r\n")
	fmt.Fprintf(&b, "--%s--\r\n", mimeBoundary)
	return []byte(b.String())
}

func sendViaConsole(msg Message) {
	log.Printf(`
===== Email (SMTP_HOST not set — printing instead of sending) =====
To: %s
Subject: %s
---
%s
=====================================================================`,
		msg.To, msg.Subject, msg.Text)
}
