package insecure

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// CreateSelfSignedCertificate creates a self-signed certificate for the key/server
func CreateSelfSignedCertificate(privateKey []byte, serverName string) (*tls.Certificate, error) {

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.Wrap(err, "generating serial number")
	}

	parsedKey, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "parsing private key")
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(5 * 365 * 24 * time.Hour),
		DNSNames:              []string{serverName},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(parsedKey), parsedKey)
	if err != nil {
		return nil, errors.Wrap(err, "creating certificate")
	}

	cert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  parsedKey,
	}
	return &cert, nil
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	case ed25519.PrivateKey:
		return k.Public().(ed25519.PublicKey)
	default:
		return nil
	}
}
