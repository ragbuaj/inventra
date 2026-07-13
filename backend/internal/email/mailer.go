package email

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	texttemplate "text/template"
)

//go:embed templates/*.html templates/*.txt
var templatesFS embed.FS

var (
	htmlTmpl = template.Must(template.ParseFS(templatesFS, "templates/*.html"))
	textTmpl = texttemplate.Must(texttemplate.ParseFS(templatesFS, "templates/*.txt"))
)

// Mailer renders account-security templates and hands them to a Sender.
type Mailer struct {
	sender Sender
}

// NewMailer builds a Mailer over the given Sender.
func NewMailer(s Sender) *Mailer { return &Mailer{sender: s} }

type resetData struct {
	Name string
	Link string
}

type changedData struct {
	Name string
}

type emailChangeData struct {
	Name string
	Link string
}

type emailChangedData struct {
	Name     string
	NewEmail string
}

func (m *Mailer) render(htmlName, textName string, data any) (html, text string, err error) {
	var hb, tb bytes.Buffer
	if err = htmlTmpl.ExecuteTemplate(&hb, htmlName, data); err != nil {
		return "", "", err
	}
	if err = textTmpl.ExecuteTemplate(&tb, textName, data); err != nil {
		return "", "", err
	}
	return hb.String(), tb.String(), nil
}

// SendPasswordReset emails a reset link valid for the token TTL.
func (m *Mailer) SendPasswordReset(ctx context.Context, to, name, link string) error {
	html, text, err := m.render("password_reset.html", "password_reset.txt", resetData{Name: name, Link: link})
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, to, "Reset Password Inventra", html, text)
}

// SendPasswordChanged notifies that the account password was changed.
func (m *Mailer) SendPasswordChanged(ctx context.Context, to, name string) error {
	html, text, err := m.render("password_changed.html", "password_changed.txt", changedData{Name: name})
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, to, "Password Inventra Diubah", html, text)
}

// SendEmailChangeVerify emails a verification link to the new address for an
// in-progress email-change request.
func (m *Mailer) SendEmailChangeVerify(ctx context.Context, to, name, link string) error {
	html, text, err := m.render("email_change_verify.html", "email_change_verify.txt", emailChangeData{Name: name, Link: link})
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, to, "Verifikasi Perubahan Email Inventra", html, text)
}

// SendEmailChanged notifies the old address that the account email was
// changed to newEmail.
func (m *Mailer) SendEmailChanged(ctx context.Context, to, name, newEmail string) error {
	html, text, err := m.render("email_changed.html", "email_changed.txt", emailChangedData{Name: name, NewEmail: newEmail})
	if err != nil {
		return err
	}
	return m.sender.Send(ctx, to, "Email Akun Inventra Diubah", html, text)
}
