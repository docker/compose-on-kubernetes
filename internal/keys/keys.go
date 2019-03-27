package keys

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math"
	"math/big"
	"time"

	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
)

const duration365d = time.Hour * 24 * 365

// SignerWithPEM is a signer implementation that has its PEM encoded representation attached
type SignerWithPEM interface {
	crypto.Signer
	PEM() []byte
}

type signerWithPEM struct {
	crypto.Signer
	pemData []byte
}

func (s signerWithPEM) PEM() []byte {
	return s.pemData
}

// CA is a certificate authority usable to generated signed certificates
type CA interface {
	PrivateKey() SignerWithPEM
	Cert() *x509.Certificate
	NewSignedCert(cfg certutil.Config, publicKey crypto.PublicKey) (*x509.Certificate, error)
}

type authority struct {
	pvk  SignerWithPEM
	cert *x509.Certificate
}

func (ca *authority) PrivateKey() SignerWithPEM {
	return ca.pvk
}

func (ca *authority) Cert() *x509.Certificate {
	return ca.cert
}

func (ca *authority) NewSignedCert(cfg certutil.Config, publicKey crypto.PublicKey) (*x509.Certificate, error) {
	serial, err := cryptorand.Int(cryptorand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}
	if len(cfg.Usages) == 0 {
		return nil, errors.New("must specify at least one ExtKeyUsage")
	}
	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.AltNames.DNSNames,
		IPAddresses:  cfg.AltNames.IPs,
		SerialNumber: serial,
		NotBefore:    ca.cert.NotBefore,
		NotAfter:     time.Now().Add(time.Hour * 24 * 365).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  cfg.Usages,
	}
	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &certTmpl, ca.cert, publicKey, ca.pvk)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

// NewSelfSignedCA creates a CA and return it with its private key
func NewSelfSignedCA(commonName string, organization []string) (CA, error) {
	pvk, err := ecdsa.GenerateKey(elliptic.P256(), cryptorand.Reader)
	if err != nil {
		return nil, err
	}
	derBytes, err := x509.MarshalECPrivateKey(pvk)
	if err != nil {
		return nil, err
	}
	keyPemBlock := &pem.Block{
		Type:  keyutil.ECPrivateKeyBlockType,
		Bytes: derBytes,
	}
	keyData := pem.EncodeToMemory(keyPemBlock)
	key := signerWithPEM{Signer: pvk, pemData: keyData}

	now := time.Now()
	tmpl := x509.Certificate{
		SerialNumber: new(big.Int).SetInt64(0),
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organization,
		},
		NotBefore:             now.UTC(),
		NotAfter:              now.Add(duration365d * 10).UTC(),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDERBytes, err := x509.CreateCertificate(cryptorand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		return nil, err
	}
	return &authority{
		cert: cert,
		pvk:  key,
	}, nil
}

// NewRSASigner generates a private key suitable for a TLS cert (client or server)
func NewRSASigner() (SignerWithPEM, error) {
	key, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	pemData, err := keyutil.MarshalPrivateKeyToPEM(key)
	if err != nil {
		return nil, err
	}
	return signerWithPEM{Signer: key, pemData: pemData}, nil
}

// EncodeCertPEM embed a certificate in a PEM block
func EncodeCertPEM(cert *x509.Certificate) []byte {
	block := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return pem.EncodeToMemory(&block)
}
