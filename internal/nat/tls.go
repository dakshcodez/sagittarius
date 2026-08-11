package nat

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"time"
)

// ALPN identifies this application's QUIC protocol to peers/tracker.
const ALPN = "sagittarius/nat"

// GenerateTLSConfig creates an ephemeral self-signed certificate for use
// as both a QUIC server and client TLS config.
//
// There is no PKI here: peer identity is established at the application
// layer (the existing network.SendHandshake/ReceiveHandshake exchange),
// not via certificate verification. InsecureSkipVerify is intentional for
// this prototype; pinning the certificate to the peer_id claimed at
// tracker registration is a real follow-up, not implemented here.
func GenerateTLSConfig() (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "sagittarius-node"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	cert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  key,
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
		NextProtos:         []string{ALPN},
	}, nil
}
