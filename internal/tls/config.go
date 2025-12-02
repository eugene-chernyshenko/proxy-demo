package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"time"

	"example.com/me/myproxy/config"
)

// NewTLSConfig создает TLS конфигурацию из конфига
func NewTLSConfig(tlsConfig *config.TLSConfig) (*tls.Config, error) {
	if tlsConfig == nil || !tlsConfig.Enabled {
		return nil, nil
	}

	if tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificate: %w", err)
		}
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
		}, nil
	}

	return nil, nil
}

// GenerateSelfSignedCert генерирует самоподписанный TLS сертификат для тестирования
func GenerateSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to generate RSA key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"MyProxy Test"},
			Country:       []string{"US"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365), // Valid for 1 year

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to create certificate: %w", err)
	}

	return tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}, nil
}

// NewTLSConfigForQUIC создает TLS конфигурацию для QUIC с поддержкой самоподписанного сертификата для тестирования
func NewTLSConfigForQUIC(tlsConfig *config.TLSConfig, nextProtos []string) (*tls.Config, error) {
	if tlsConfig == nil || !tlsConfig.Enabled {
		// Для тестирования генерируем самоподписанный сертификат
		cert, err := GenerateSelfSignedCert()
		if err != nil {
			return nil, fmt.Errorf("failed to generate self-signed certificate: %w", err)
		}
		return &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   nextProtos,
		}, nil
	}

	cfg, err := NewTLSConfig(tlsConfig)
	if err != nil {
		return nil, err
	}

	if cfg != nil {
		cfg.NextProtos = nextProtos
	}

	return cfg, nil
}

