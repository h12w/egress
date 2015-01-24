package local

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path"
	"sync"
)

func fakeHTTPSHandeshake(conn net.Conn, host string, pool *certPool) (net.Conn, error) {
	if _, err := conn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
		return nil, err
	}
	// every host should have its own cert
	cert, err := pool.get(host)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		ServerName:   host,
	}
	tls := tls.Server(conn, config)
	if err := tls.Handshake(); err != nil {
		return nil, err
	}
	return tls, nil
}

type certPool struct {
	ca    *tls.Certificate
	dir   string
	data  map[string]*tls.Certificate
	mutex sync.Mutex
}

func newCertPool(dir string) (*certPool, error) {
	poolDir := path.Join(dir, "certs")
	if err := os.MkdirAll(poolDir, 0755); err != nil && !os.IsExist(err) {
		return nil, err
	}
	ca, err := tls.LoadX509KeyPair(path.Join(dir, "crt"), path.Join(dir, "key"))
	if err != nil {
		return nil, err
	}

	return &certPool{
		dir:  poolDir,
		ca:   &ca,
		data: make(map[string]*tls.Certificate),
	}, nil
}

func (pool *certPool) get(host string) (*tls.Certificate, error) {
	pool.mutex.Lock()
	defer pool.mutex.Unlock()

	if c, ok := pool.data[host]; ok {
		return c, nil
	}

	certFile := path.Join(pool.dir, host+".crt")
	der, err := ioutil.ReadFile(certFile)
	if err == nil {
		rcert, err := tls.X509KeyPair(pool.ca.Certificate[0], der)
		if err == nil {
			pool.data[host] = &rcert
			return &rcert, err
		}
	}

	cert, err := pool.gen(host)
	if err != nil {
		return nil, err
	}
	pool.data[host] = cert
	saveCertFile(cert, certFile)
	return cert, nil
}

func saveCertFile(cert *tls.Certificate, file string) {
	f, err := os.Create(file)
	if err != nil {
		return
	}
	defer f.Close()
	for _, c := range cert.Certificate {
		err = pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: c})
		if err != nil {
			defer os.Remove(file)
			break
		}
	}
}

func (pool *certPool) gen(host string) (*tls.Certificate, error) {
	signer, err := x509.ParseCertificate(pool.ca.Certificate[0])
	if err != nil {
		return nil, err
	}
	signer.Subject.CommonName = host

	hash := sha1.Sum([]byte(host))
	signee := &x509.Certificate{
		SerialNumber:          new(big.Int).SetBytes(hash[:]),
		Issuer:                signer.Issuer,
		Subject:               signer.Subject,
		NotBefore:             signer.NotBefore,
		NotAfter:              signer.NotAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	key := pool.ca.PrivateKey.(*rsa.PrivateKey)
	der, err := x509.CreateCertificate(rand.Reader, signee, signer, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{der, pool.ca.Certificate[0]},
		PrivateKey:  key,
	}, nil
}
