package courier

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/textproto"
	"strings"
	"sync"
	"testing"

	"blitiri.com.ar/go/chasquid/internal/testlib"
)

type fakeServer struct {
	addr    string
	wg      *sync.WaitGroup
	rootCAs *x509.CertPool
}

// Fake server, to test SMTP out.
func newFakeServer(t *testing.T, responses map[string]string) *fakeServer {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("fake server listen: %v", err)
	}

	clientTLSConfig, serverTLSConfig := testlib.TLSConfig()

	srv := &fakeServer{
		addr:    l.Addr().String(),
		wg:      &sync.WaitGroup{},
		rootCAs: clientTLSConfig.RootCAs,
	}

	srv.wg.Add(1)

	go func() {
		defer srv.wg.Done()
		defer l.Close()

		var c net.Conn
		var err error
		c, err = l.Accept()
		if err != nil {
			panic(err)
		}
		defer c.Close()

		t.Logf("fakeServer got connection")

		r := textproto.NewReader(bufio.NewReader(c))
		c.Write([]byte(responses["_welcome"]))
		for {
			line, err := r.ReadLine()
			if err != nil {
				t.Logf("fakeServer exiting: %v\n", err)
				return
			}

			t.Logf("fakeServer read: %q\n", line)
			c.Write([]byte(responses[line]))

			if line == "DATA" {
				_, err = r.ReadDotBytes()
				if err != nil {
					t.Logf("fakeServer exiting: %v\n", err)
					return
				}
				c.Write([]byte(responses["_DATA"]))
			} else if line == "STARTTLS" && strings.HasPrefix(responses[line], "220 ") {
				tlsconn := tls.Server(c, serverTLSConfig)
				defer tlsconn.Close()

				if err = tlsconn.Handshake(); err != nil {
					t.Logf("fakeServer error in STARTTLS: %v", err)
					return
				}
				c = tlsconn
				r = textproto.NewReader(bufio.NewReader(c))
			}
		}
	}()

	return srv
}

func makeResp(as ...string) map[string]string {
	m := map[string]string{}
	for i := 0; i < len(as); i += 2 {
		m[as[i]] = as[i+1]
	}
	return m
}
