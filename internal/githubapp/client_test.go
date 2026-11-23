package githubapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestAppJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	c := Client{AppID: "123", PrivateKeyPEM: raw, Now: func() time.Time { return time.Unix(1700000000, 0) }}
	jwt, err := c.appJWT()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(jwt, ".") != 2 {
		t.Fatalf("malformed JWT: %q", jwt)
	}
}
