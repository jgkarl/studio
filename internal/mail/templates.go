package mail

import "fmt"

// Plain, unstyled text + minimal HTML — matches the app's no-framework approach elsewhere rather
// than pulling in an email-templating library for two short messages.

func VerifyEmailMessage(userName, link string) Message {
	text := fmt.Sprintf(`Hi %s,

Confirm your email address to finish setting up your Stuudio account:
%s

This link expires in 24 hours. If you didn't request this, you can ignore this email.`, userName, link)

	html := fmt.Sprintf(`<p>Hi %s,</p><p>Confirm your email address to finish setting up your Stuudio account:</p><p><a href="%s">%s</a></p><p>This link expires in 24 hours. If you didn't request this, you can ignore this email.</p>`, userName, link, link)

	return Message{Subject: "Confirm your Stuudio account", Text: text, HTML: html}
}

func ResetPasswordMessage(userName, link string) Message {
	text := fmt.Sprintf(`Hi %s,

Someone requested a password reset for your Stuudio account. Reset it here:
%s

This link expires in 1 hour. If you didn't request this, you can ignore this email — your password won't change.`, userName, link)

	html := fmt.Sprintf(`<p>Hi %s,</p><p>Someone requested a password reset for your Stuudio account. Reset it here:</p><p><a href="%s">%s</a></p><p>This link expires in 1 hour. If you didn't request this, you can ignore this email — your password won't change.</p>`, userName, link, link)

	return Message{Subject: "Reset your Stuudio password", Text: text, HTML: html}
}
