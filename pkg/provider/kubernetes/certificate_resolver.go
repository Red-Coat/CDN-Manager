/*
Copyright 2021 Red Coat Development Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This loads a certificate secret from the kubernetes api, performs
// serveral checks on it, and returns it as a parsed Certificate
type CertificateResolver struct {
	client.Client
	secret   corev1.Secret
	resolved *Certificate
}

// Generic Wrapper for the Private key - we use this because the private
// key parsing code returns various unrelated structs depending on the
// key type
type PrivateKey struct {
	// The raw ASN.1 DER contents of the certificate
	Raw []byte

	// The parsed certificate. One of:
	// - rsa.PrivateKey
	// - ecdsa.PrivateKey
	// - ed25519.PrivateKey
	//
	// These do not share any common interfaces, so we have to just type
	// this as a generic interface{}
	Parsed interface{}
}

// Holds a complete loaded and parsed certificate
type Certificate struct {
	// The certificate on its own
	Certificate *x509.Certificate

	// Any certificates up the certification path for this certificate
	Chain []*x509.Certificate

	// The private key for this certificate
	Key PrivateKey
}

// Loads the the secret given and parses it as a
// kubernetes.io/tls-secret
func (c *CertificateResolver) Resolve(secretRef client.ObjectKey) (*Certificate, error) {
	if err := c.load(secretRef); err != nil {
		return nil, err
	}

	c.resolved = &Certificate{}
	c.parseCrt()
	c.parseKey()

	return c.resolved, nil
}

// Loads a secret and checks that it is of the type
// kubernetes.io/tls-cert
func (c *CertificateResolver) load(secretRef client.ObjectKey) error {
	c.Get(context.TODO(), secretRef, &c.secret)

	if c.secret.Type == "" {
		return fmt.Errorf("Could not find the TLS secret \"%v\"", secretRef.Name)
	} else if c.secret.Type != corev1.SecretTypeTLS {
		return fmt.Errorf(
			"TLS secret \"%v\" has an invalid type. Expecting %v, got %v",
			secretRef.Name,
			corev1.SecretTypeTLS,
			c.secret.Type,
		)
	}

	return nil
}

// Checks that the given data field exists on the certificate and, if it
// does, returns it
func (c *CertificateResolver) getData(name string) ([]byte, error) {
	data := c.secret.Data[name]
	if len(data) == 0 {
		return nil, fmt.Errorf(
			"TLS secret \"%v\" does not have the required data \"%v\"",
			c.secret.Name,
			name,
		)
	}

	return data, nil
}

// Loads the tls.crt section of the secret and parses it into the
// resolved Certificate
func (c *CertificateResolver) parseCrt() error {
	raw, err := c.getData("tls.crt")
	if err != nil {
		return err
	}

	for len(raw) > 0 {
		var block *pem.Block
		block, raw = pem.Decode(raw)
		cert, err := x509.ParseCertificate(block.Bytes)

		if err != nil {
			return err
		} else if c.resolved.Certificate == nil {
			c.resolved.Certificate = cert
		} else {
			c.resolved.Chain = append(c.resolved.Chain, cert)
		}
	}

	return nil
}

// Loads the tls.eky section of the secret and parses it into the
// resolved Certificate
func (c *CertificateResolver) parseKey() error {
	key, err := c.getData("tls.key")
	if err != nil {
		return err
	}

	block, _ := pem.Decode(key)
	c.resolved.Key.Raw = block.Bytes

	if block.Type == "RSA PRIVATE KEY" {
		c.resolved.Key.Parsed, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	} else if block.Type == "PRIVATE KEY" {
		c.resolved.Key.Parsed, err = x509.ParsePKCS1PrivateKey(block.Bytes)
	} else {
		return fmt.Errorf(
			"TLS secret \"%v\"'s private key was invalid",
			c.secret.Name,
		)
	}
	if err != nil {
		return err
	}

	return nil
}
