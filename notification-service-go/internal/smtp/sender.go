package smtp

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"

	"Banka1Back/notification-service-go/internal/config"
)

// Sender dispatches a single email message over SMTP with STARTTLS support.
type Sender struct {
	host      string
	port      int
	username  string
	password  string
	from      string
	startTLS  bool
	insecure  bool
	tlsConfig *tls.Config
}

func NewSender(cfg config.SMTPConfig) *Sender {
	return &Sender{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.Username,
		startTLS: cfg.StartTLS,
		insecure: cfg.InsecureSkipVerify,
	}
}

// SendEmail delivers one email via SMTP.
//
// Error classification:
//   - *MailAuthError   — server rejected credentials (535), non-retryable.
//   - *PermanentSMTPError — other 5xx responses, non-retryable.
//   - All other errors — network, TLS, 4xx transient, retryable.
func (s *Sender) SendEmail(to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	from := s.from
	if strings.TrimSpace(from) == "" {
		from = s.username
	}

	msg := buildRFC2822Message(from, to, subject, body)

	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	err := s.send(addr, auth, from, []string{to}, []byte(msg))
	if err == nil {
		return nil
	}
	return classifyError(err)
}

func (s *Sender) send(addr string, auth smtp.Auth, from string, recipients []string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()

	if s.startTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			tlsCfg := s.tlsConfig
			if tlsCfg == nil {
				tlsCfg = &tls.Config{ServerName: s.host, RootCAs: systemCertPool(), InsecureSkipVerify: s.insecure} //nolint:gosec
			}
			if err := c.StartTLS(tlsCfg); err != nil {
				return err
			}
		}
	}

	if auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err := c.Rcpt(recipient); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func systemCertPool() *x509.CertPool {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil
	}
	return pool
}

// MailAuthError indicates that the SMTP server rejected our credentials.
// Non-retryable — the same credentials will always fail.
type MailAuthError struct {
	Cause error
}

func (e *MailAuthError) Error() string { return "SMTP authentication failed: " + e.Cause.Error() }
func (e *MailAuthError) Unwrap() error { return e.Cause }

// PermanentSMTPError wraps a server-side 5xx error that is not an auth failure.
type PermanentSMTPError struct {
	Code int
	Msg  string
}

func (e *PermanentSMTPError) Error() string {
	return fmt.Sprintf("permanent SMTP failure %d: %s", e.Code, e.Msg)
}

// IsRetryable returns true for transient SMTP failures (4xx, network, TLS)
// and false for permanent failures (auth errors, 5xx responses).
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var authErr *MailAuthError
	if errors.As(err, &authErr) {
		return false
	}
	var permErr *PermanentSMTPError
	if errors.As(err, &permErr) {
		return false
	}
	return true
}

func classifyError(err error) error {
	var textErr *textproto.Error
	if errors.As(err, &textErr) {
		if textErr.Code == 535 {
			return &MailAuthError{Cause: err}
		}
		if textErr.Code >= 500 {
			return &PermanentSMTPError{Code: textErr.Code, Msg: textErr.Msg}
		}
		return err
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "535") ||
		strings.Contains(msg, "authentication") ||
		strings.Contains(msg, "credentials") {
		return &MailAuthError{Cause: err}
	}
	return err
}

func buildRFC2822Message(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: ")
	sb.WriteString(from)
	sb.WriteString("\r\nTo: ")
	sb.WriteString(to)
	sb.WriteString("\r\nSubject: ")
	sb.WriteString(subject)
	sb.WriteString("\r\nMIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	return sb.String()
}

func (s *Sender) dialTLS(addr string) (*smtp.Client, error) {
	tlsCfg := s.tlsConfig
	if tlsCfg == nil {
		tlsCfg = &tls.Config{ServerName: s.host} //nolint:gosec
	}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("TLS dial %s: %w", addr, err)
	}
	host, _, _ := net.SplitHostPort(addr)
	return smtp.NewClient(conn, host)
}
