package courier

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	netsmtp "net/smtp"
	"net/url"
	"time"

	"blitiri.com.ar/go/chasquid/internal/expvarom"
	"blitiri.com.ar/go/chasquid/internal/smtp"
	"blitiri.com.ar/go/chasquid/internal/trace"
)

var (
	// Timeouts for smarthost delivery.
	shDialTimeout  = 1 * time.Minute
	shTotalTimeout = 10 * time.Minute
)

// Exported variables.
var (
	shAttempts = expvarom.NewInt("chasquid/smarthostOut/attempts",
		"count of attempts to deliver via smarthost")
	shErrors = expvarom.NewMap("chasquid/smarthostOut/errors",
		"reason", "count of smarthost delivery errors, per reason")
	shSuccess = expvarom.NewInt("chasquid/smarthostOut/success",
		"count of successful delivering via smarthost")
)

// SmartHost delivers remote mail via smarthost relaying.
type SmartHost struct {
	HelloDomain string
	URL         url.URL

	// For testing.
	rootCAs *x509.CertPool
}

// Deliver an email. On failures, returns an error, and whether or not it is
// permanent.
func (s *SmartHost) Deliver(from string, to string, data []byte) (error, bool) {
	tr := trace.New("Courier.SmartHost", to)
	defer tr.Finish()
	tr.Debugf("%s  ->  %s", from, to)
	shAttempts.Add(1)

	conn, onTLS, err := s.dial()
	if err != nil {
		shErrors.Add("dial", 1)
		return tr.Errorf("Could not dial %q: %v", s.URL.Host, err), false
	}

	defer conn.Close()
	conn.SetDeadline(time.Now().Add(shTotalTimeout))

	host, _, _ := net.SplitHostPort(s.URL.Host)

	c, err := smtp.NewClient(conn, host)
	if err != nil {
		shErrors.Add("client", 1)
		return tr.Errorf("Error creating client: %v", err), false
	}

	if err = c.Hello(s.HelloDomain); err != nil {
		shErrors.Add("hello", 1)
		return tr.Errorf("Error saying hello: %v", err), false
	}

	if !onTLS {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			shErrors.Add("starttls-support", 1)
			return tr.Errorf("Server does not support STARTTLS"), false
		}

		config := &tls.Config{
			ServerName: host,
			RootCAs:    s.rootCAs,
		}
		if err = c.StartTLS(config); err != nil {
			shErrors.Add("starttls-exchange", 1)
			return tr.Errorf("Error in STARTTLS: %v", err), false
		}
	}

	if s.URL.User != nil {
		user := s.URL.User.Username()
		password, _ := s.URL.User.Password()
		auth := netsmtp.PlainAuth("", user, password, host)
		if err = c.Auth(auth); err != nil {
			shErrors.Add("auth", 1)
			return tr.Errorf("AUTH error: %v", err), false
		}
	}

	// smtp.Client.Mail will add the <> for us when the address is empty.
	if from == "<>" {
		from = ""
	}

	if err = c.MailAndRcpt(from, to); err != nil {
		shErrors.Add("mail", 1)
		return tr.Errorf("MAIL+RCPT %v", err), smtp.IsPermanent(err)
	}

	w, err := c.Data()
	if err != nil {
		shErrors.Add("data", 1)
		return tr.Errorf("DATA %v", err), smtp.IsPermanent(err)
	}
	_, err = w.Write(data)
	if err != nil {
		shErrors.Add("dataw", 1)
		return tr.Errorf("DATA writing: %v", err), smtp.IsPermanent(err)
	}

	err = w.Close()
	if err != nil {
		shErrors.Add("close", 1)
		return tr.Errorf("DATA closing %v", err), smtp.IsPermanent(err)
	}

	_ = c.Quit()
	tr.Debugf("done")
	shSuccess.Add(1)

	return nil, false
}

func (s *SmartHost) dial() (conn net.Conn, onTLS bool, err error) {
	dialer := &net.Dialer{Timeout: shDialTimeout}

	if s.URL.Scheme == "tls" {
		onTLS = true
		config := &tls.Config{
			RootCAs: s.rootCAs,
		}
		conn, err = tls.DialWithDialer(dialer, "tcp", s.URL.Host, config)
	} else {
		onTLS = false
		conn, err = dialer.Dial("tcp", s.URL.Host)
	}
	return
}
