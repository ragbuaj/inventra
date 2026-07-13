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
