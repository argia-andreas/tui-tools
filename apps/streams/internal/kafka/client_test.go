package kafka

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"streams/internal/config"
)

func TestVerifiedTLSMutualTLSAndSASLConstruction(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test-only"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	pk, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert := write("cert.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyfile := write("key.pem", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}))
	password := write("password", []byte("test-only-password\n"))
	for _, mechanism := range []string{"", "PLAIN", "SCRAM-SHA-256", "SCRAM-SHA-512"} {
		p := config.Profile{Name: "secure", Brokers: []string{"broker.example:9093"}, TLS: true, CAFile: cert, CertFile: cert, KeyFile: keyfile, SASL: mechanism}
		if mechanism != "" {
			p.Username = "reader"
			p.PasswordFile = password
		}
		c, err := New(p)
		if err != nil {
			t.Fatal(err)
		}
		cfg, ok := c.metadata.OptValue(kgo.DialTLSConfig).(*tls.Config)
		if !ok || cfg.InsecureSkipVerify || cfg.MinVersion < tls.VersionTLS12 || cfg.RootCAs == nil || len(cfg.Certificates) != 1 {
			t.Fatal("verified mTLS configuration missing")
		}
		if c.metadata.OptValue(kgo.ConsumerGroup) != "" || c.metadata.OptValue(kgo.AllowAutoTopicCreation) != false || c.metadata.OptValue(kgo.MaxConcurrentFetches) != 0 {
			t.Fatal("unsafe broker client defaults")
		}
		if c.metadata.OptValue(kgo.BrokerMaxReadBytes) != int32(16<<20) || c.metadata.OptValue(kgo.MaxDecompressBatchBytes) != 8<<20 {
			t.Fatal("transport/decompression bounds missing")
		}
		c.Close()
	}
	bad := config.Profile{Name: "bad", Brokers: []string{"broker.example:9093"}, TLS: true, CAFile: write("bad-ca", []byte("not a certificate"))}
	if _, err := New(bad); err == nil {
		t.Fatal("invalid CA accepted")
	}
	bad.CAFile = cert
	bad.CertFile = cert
	bad.KeyFile = write("bad-key", []byte("not a private key"))
	if _, err := New(bad); err == nil {
		t.Fatal("invalid mTLS key accepted")
	}
	bad.CertFile = ""
	bad.KeyFile = ""
	bad.SASL = "PLAIN"
	bad.Username = "reader"
	bad.PasswordFile = write("empty", nil)
	if _, err := New(bad); err == nil {
		t.Fatal("empty password accepted")
	}
}
