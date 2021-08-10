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

package resolver

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

// Holds the Original and Parsed representations of the certificate
type CertificateWrapper struct {
	Encoded []byte

	Parsed *x509.Certificate
}

// Holds a complete loaded and parsed certificate
type Certificate struct {
	// The certificate on its own
	Certificate CertificateWrapper

	// Any certificates up the certification path for this certificate
	Chain []byte

	// The private key for this certificate
	Key []byte
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

	block, rest := pem.Decode(raw)
	cert, err := x509.ParseCertificate(block.Bytes)
	c.resolved.Certificate = CertificateWrapper{
		Encoded: raw[:len(raw)-len(rest)],
		Parsed:  cert,
	}
	c.resolved.Chain = rest

	return err
}

// Loads the tls.eky section of the secret and parses it into the
// resolved Certificate
func (c *CertificateResolver) parseKey() error {
	key, err := c.getData("tls.key")
	if err != nil {
		return err
	}

	block, rest := pem.Decode(key)
	c.resolved.Key = key[:len(key)-len(rest)]

	if block.Type != "RSA PRIVATE KEY" && block.Type != "PRIVATE KEY" {
		return fmt.Errorf(
			"TLS secret \"%v\"'s private key was invalid",
			c.secret.Name,
		)
	}

	return err
}
