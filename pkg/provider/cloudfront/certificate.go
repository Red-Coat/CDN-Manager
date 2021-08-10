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

package cloudfront

import (
	"crypto/x509"
	"encoding/pem"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acm"

	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	"git.redcoat.dev/cdn/pkg/provider/kubernetes"
)

type CertificateProvider struct {
	Client      *acm.ACM
	Status      *api.DistributionStatus
	Certificate *kubernetes.Certificate
}

func (c *CertificateProvider) Reconcile() error {
	if c.Status.CloudFront.CertificateArn != "" {
		return c.Check()
	} else {
		return c.Create()
	}
}

func (c *CertificateProvider) Check() error {
	info, err := c.Client.GetCertificate(&acm.GetCertificateInput{
		CertificateArn: aws.String(c.Status.CloudFront.CertificateArn),
	})

	if is, _ := isAwsError(err, "ResourceNotFoundException"); is {
		c.Status.CloudFront.CertificateArn = ""
		return c.Create()
	} else if err != nil {
		return err
	}

	// We just need to double check that the serial numbers match
	block, _ := pem.Decode([]byte(*info.Certificate))
	cert, _ := x509.ParseCertificate(block.Bytes)
	if c.Certificate.Certificate.Parsed.SerialNumber.Cmp(cert.SerialNumber) != 0 {
		return c.Create()
	}

	return nil
}

func (c *CertificateProvider) Create() error {
	var arn *string
	if c.Status.CloudFront.CertificateArn != "" {
		arn = aws.String(c.Status.CloudFront.CertificateArn)
	}

	info, err := c.Client.ImportCertificate(&acm.ImportCertificateInput{
		Certificate:      c.Certificate.Certificate.Encoded,
		CertificateChain: c.Certificate.Chain,
		PrivateKey:       c.Certificate.Key,
		CertificateArn:   arn,
	})

	if err != nil {
		return err
	}

	c.Status.CloudFront.CertificateArn = *info.CertificateArn
	return nil
}
