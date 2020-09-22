package courier

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func newSmartHost(t *testing.T, addr string) *SmartHost {
	return &SmartHost{
		HelloDomain: "hello",
		URL: url.URL{
			Scheme: "smtp",
			Host:   addr,
		},
	}
}

func TestSmartHost(t *testing.T) {
	// Shorten the total timeout, so the test fails quickly if the protocol
	// gets stuck.
	shTotalTimeout = 3 * time.Second

	responses := map[string]string{
		"_welcome":   "220 welcome\n",
		"EHLO hello": "250-ehlo ok\n250 STARTTLS AUTH HELP\n",
		"STARTTLS":   "220 tls ok\n",

		// Auth corresponds to the user and password below.
		"AUTH PLAIN AHVzZXIAcGFzc3dvcmQ=": "235 auth ok\n",

		"MAIL FROM:<me@me>": "250 mail ok\n",
		"RCPT TO:<to@to>":   "250 rcpt ok\n",
		"DATA":              "354 send data\n",
		"_DATA":             "250 data ok\n",
		"QUIT":              "250 quit ok\n",
	}
	srv := newFakeServer(t, responses)

	sh := newSmartHost(t, srv.addr)
	sh.URL.User = url.UserPassword("user", "password")
	sh.rootCAs = srv.rootCAs
	err, _ := sh.Deliver("me@me", "to@to", []byte("data"))
	if err != nil {
		t.Errorf("deliver failed: %v", err)
	}

	srv.wg.Wait()
}

func TestSmartHostBadAuth(t *testing.T) {
	// Shorten the total timeout, so the test fails quickly if the protocol
	// gets stuck.
	shTotalTimeout = 3 * time.Second

	responses := map[string]string{
		"_welcome":   "220 welcome\n",
		"EHLO hello": "250-ehlo ok\n250-STARTTLS\n250 AUTH PLAIN\n",
		"STARTTLS":   "220 tls ok\n",

		// Auth corresponds to the user and password below.
		"AUTH PLAIN AHVzZXIAcGFzc3dvcmQ=": "454 auth error\n",

		// The client will use an "*" to abort the auth on errors.
		"*": "501 invalid command\n",

		"QUIT": "250 quit ok\n",
	}
	srv := newFakeServer(t, responses)

	sh := newSmartHost(t, srv.addr)
	sh.URL.User = url.UserPassword("user", "password")
	sh.rootCAs = srv.rootCAs
	err, _ := sh.Deliver("me@me", "to@to", []byte("data"))
	if !strings.HasPrefix(err.Error(), "AUTH error: 454 auth error") {
		t.Errorf("expected error in AUTH, got %q", err)
	}

	srv.wg.Wait()
}

func TestSmartHostBadCert(t *testing.T) {
	// Shorten the total timeout, so the test fails quickly if the protocol
	// gets stuck.
	shTotalTimeout = 3 * time.Second

	responses := map[string]string{
		"_welcome":   "220 welcome\n",
		"EHLO hello": "250-ehlo ok\n250 STARTTLS\n",
		"STARTTLS":   "220 tls ok\n",
	}
	srv := newFakeServer(t, responses)

	sh := newSmartHost(t, srv.addr)
	// We do NOT set the root CA to our test server's certificate, so we
	// expect the STARTTLS negotiation to fail.
	err, _ := sh.Deliver("me@me", "to@to", []byte("data"))
	if !strings.HasPrefix(err.Error(), "Error in STARTTLS:") {
		t.Errorf("expected error in STARTTLS, got %q", err)
	}

	srv.wg.Wait()
}

func TestSmartHostErrors(t *testing.T) {
	// Shorten the total timeout, so the test fails quickly if the protocol
	// gets stuck.
	shTotalTimeout = 1 * time.Second

	cases := []struct {
		responses map[string]string
		errPrefix string
	}{
		// First test: hang response, should fail due to timeout.
		{
			makeResp("_welcome", "220 no newline"),
			"",
		},

		// No STARTTLS support.
		{
			makeResp(
				"_welcome", "220 rcpt to not allowed\n",
				"EHLO hello", "250-ehlo ok\n250 HELP\n",
			),
			"Server does not support STARTTLS",
		},

		// MAIL FROM not allowed.
		{
			makeResp(
				"_welcome", "220 mail from not allowed\n",
				"EHLO hello", "250-ehlo ok\n250 STARTTLS\n",
				"STARTTLS", "220 tls ok\n",
				"MAIL FROM:<me@me>", "501 mail error\n",
			),
			"MAIL+RCPT 501 mail error",
		},

		// RCPT TO not allowed.
		{
			makeResp(
				"_welcome", "220 rcpt to not allowed\n",
				"EHLO hello", "250-ehlo ok\n250 STARTTLS\n",
				"STARTTLS", "220 tls ok\n",
				"MAIL FROM:<me@me>", "250 mail ok\n",
				"RCPT TO:<to@to>", "501 rcpt error\n",
			),
			"MAIL+RCPT 501 rcpt error",
		},

		// DATA error.
		{
			makeResp(
				"_welcome", "220 data error\n",
				"EHLO hello", "250-ehlo ok\n250 STARTTLS\n",
				"STARTTLS", "220 tls ok\n",
				"MAIL FROM:<me@me>", "250 mail ok\n",
				"RCPT TO:<to@to>", "250 rcpt ok\n",
				"DATA", "554 data error\n",
			),
			"DATA 554 data error",
		},

		// DATA response error.
		{
			makeResp(
				"_welcome", "220 data error\n",
				"EHLO hello", "250-ehlo ok\n250 STARTTLS\n",
				"STARTTLS", "220 tls ok\n",
				"MAIL FROM:<me@me>", "250 mail ok\n",
				"RCPT TO:<to@to>", "250 rcpt ok\n",
				"DATA", "354 send data\n",
				"_DATA", "551 data response error\n",
			),
			"DATA closing 551 data response error",
		},
	}

	for _, c := range cases {
		srv := newFakeServer(t, c.responses)
		sh := newSmartHost(t, srv.addr)
		sh.rootCAs = srv.rootCAs

		err, _ := sh.Deliver("me@me", "to@to", []byte("data"))
		if err == nil {
			t.Errorf("deliver not failed in case %q: %v",
				c.responses["_welcome"], err)
			continue
		}
		t.Logf("failed as expected: %v", err)

		if !strings.HasPrefix(err.Error(), c.errPrefix) {
			t.Errorf("expected error prefix %q, got %q",
				c.errPrefix, err)
		}

		srv.wg.Wait()
	}
}

func TestSmartHostDialError(t *testing.T) {
	sh := newSmartHost(t, "localhost:1")
	err, permanent := sh.Deliver("me@me", "to@to", []byte("data"))
	if err == nil {
		t.Errorf("delivery worked, expected failure")
	}
	if permanent {
		t.Errorf("expected transient failure, got permanent (%v)", err)
	}
	t.Logf("got transient failure, as expected: %v", err)
}
