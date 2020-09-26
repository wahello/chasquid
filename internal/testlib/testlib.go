// Package testlib provides common test utilities.
package testlib

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// MustTempDir creates a temporary directory, or dies trying.
func MustTempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "testlib_")
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chdir(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("test directory: %q", dir)
	return dir
}

// RemoveIfOk removes the given directory, but only if we have not failed. We
// want to keep the failed directories for debugging.
func RemoveIfOk(t *testing.T, dir string) {
	// Safeguard, to make sure we only remove test directories.
	// This should help prevent accidental deletions.
	if !strings.Contains(dir, "testlib_") {
		panic("invalid/dangerous directory")
	}

	if !t.Failed() {
		os.RemoveAll(dir)
	}
}

// Rewrite a file with the given contents.
func Rewrite(t *testing.T, path, contents string) error {
	// Safeguard, to make sure we only mess with test files.
	if !strings.Contains(path, "testlib_") {
		panic("invalid/dangerous path")
	}

	err := ioutil.WriteFile(path, []byte(contents), 0600)
	if err != nil {
		t.Errorf("failed to rewrite file: %v", err)
	}

	return err
}

// GetFreePort returns a free TCP port. This is hacky and not race-free, but
// it works well enough for testing purposes.
func GetFreePort() string {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}
	defer l.Close()
	return l.Addr().String()
}

// WaitFor f to return true (returns true), or d to pass (returns false).
func WaitFor(f func() bool, d time.Duration) bool {
	start := time.Now()
	for time.Since(start) < d {
		if f() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

type deliverRequest struct {
	From string
	To   string
	Data []byte
}

// TestCourier never fails, and always remembers everything.
type TestCourier struct {
	wg       sync.WaitGroup
	Requests []*deliverRequest
	ReqFor   map[string]*deliverRequest
	sync.Mutex
}

// Deliver the given mail (saving it in tc.Requests).
func (tc *TestCourier) Deliver(from string, to string, data []byte) (error, bool) {
	defer tc.wg.Done()
	dr := &deliverRequest{from, to, data}
	tc.Lock()
	tc.Requests = append(tc.Requests, dr)
	tc.ReqFor[to] = dr
	tc.Unlock()
	return nil, false
}

// Expect i mails to be delivered.
func (tc *TestCourier) Expect(i int) {
	tc.wg.Add(i)
}

// Wait until all mails have been delivered.
func (tc *TestCourier) Wait() {
	tc.wg.Wait()
}

// NewTestCourier returns a new, empty TestCourier instance.
func NewTestCourier() *TestCourier {
	return &TestCourier{
		ReqFor: map[string]*deliverRequest{},
	}
}

type dumbCourier struct{}

func (c dumbCourier) Deliver(from string, to string, data []byte) (error, bool) {
	return nil, false
}

// DumbCourier always succeeds delivery, and ignores everything.
var DumbCourier = dumbCourier{}

func TLSConfig() (client, server *tls.Config) {
	cert, err := tls.X509KeyPair(testCert, testKey)
	if err != nil {
		panic(fmt.Sprintf("error creating key pair: %v", err))
	}

	server = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	srvCert, err := x509.ParseCertificate(server.Certificates[0].Certificate[0])
	if err != nil {
		panic(fmt.Sprintf("error extracting server cert: %v", err))
	}

	pool := x509.NewCertPool()
	pool.AddCert(srvCert)
	client = &tls.Config{
		RootCAs: pool,
	}

	return
}

// PEM-encoded TLS certs, for "localhost", "127.0.0.1" and "[::1]".
// Generated with:
// go run /usr/share/go-1.14/src/crypto/tls/generate_cert.go  --rsa-bits 1024 --host 127.0.0.1,::1,localhost --ca --start-date "Jan 1 00:00:00 1970" --duration=1000000h
var testCert = []byte(`-----BEGIN CERTIFICATE-----
MIICETCCAXqgAwIBAgIQZRK1BVeALoVrF03BQur3kzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIGfMA0GCSqGSIb3DQEBAQUAA4GNADCB
iQKBgQCseGPB9aV9c/c/MbRVxwjsG5fqsb+/HnTSh1QDbGnVvCDoJZg4wJv3AEh1
s9+A12+/ImIpB8I+8sr0ErPfGQ3fAJFx+TgQ6xmyDtNWjkTPHt6AF3cb2jv0rmze
Dpa1vFXe0FwiZ2d9d1ZGvw1sPIugVAyGjW98bCxw42PXGd4s0wIDAQABo2YwZDAO
BgNVHQ8BAf8EBAMCAqQwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDwYDVR0TAQH/BAUw
AwEB/zAsBgNVHREEJTAjgglsb2NhbGhvc3SHBH8AAAGHEAAAAAAAAAAAAAAAAAAA
AAEwDQYJKoZIhvcNAQELBQADgYEASUE1j+hQT7LgKYaP0w1itfcSZGhR1ZGnMThZ
iiPnHt6ZLINZ39x2P/71KJYZklpJgewGBVRqMNTIW6hAa3UU7giQHQDDwSCtH4Zf
C4WOwq3LouX8rLqZwq8W6ETnpbSUDEslhVR2IdufcLH947yoWbLuUc30SJET6dZq
Fd+Ux3o=
-----END CERTIFICATE-----`)

var testKey = []byte(`-----BEGIN PRIVATE KEY-----
MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBAKx4Y8H1pX1z9z8x
tFXHCOwbl+qxv78edNKHVANsadW8IOglmDjAm/cASHWz34DXb78iYikHwj7yyvQS
s98ZDd8AkXH5OBDrGbIO01aORM8e3oAXdxvaO/SubN4OlrW8Vd7QXCJnZ313Vka/
DWw8i6BUDIaNb3xsLHDjY9cZ3izTAgMBAAECgYAgOia52YLg3Eh5AHqoBJcAN2+9
pRUlSzWdGThzo1BrZcnoVw4InMUH9H+VrtS2qIry9iPNcuuzA380+EGwEGhs0o/q
jHd4HtAgPK0zI/DalbVRzkiU9Qjqq2CHMpuJYIh+S2TlGHDUkShdnKi3RCgV0FC6
B49JDnMrcOIyGLKFMQJBANSh86c1ZesTMhkw0tJh4DX0pSqfPI2aSXaRBAvUBUcO
A/6EYlML5r2Fp6TyqxXk4+Dou9oG3yaUUoEA02Osyv0CQQDPpXefcP28PzamivrI
o3SQMhIJ2dpqZw7YIbQQ1cE46Q4fVTXpq/eut0TQTcOEEUR9XzIsa3x/qbkJbHFm
vegPAkEAs69gTY7sX6jLD0qY/bxEUpQ49zm1XBxjtFR7zNsQ0qjfazfIN1G5XbMS
pmuDdG8Gu0sxY9+mt91jkyx1dqfQqQJARnyuAdLSX1+6BojxHsDV5ckJdIyeZzY6
xMWUIY7eO5ppb9t2JK96sbWGx4tOTnuqG0EAgDGwnomXxYopaK4YowJBAIjMMW31
h33KQZJ7ON4pX0+AP2yeHZsbdTXpRuVXeRgoZUK7mo0nWuHFy5etz10GVw4AZtze
cfH9gZHF5jzmpyo=
-----END PRIVATE KEY-----`)
