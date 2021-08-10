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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/cloudfront"

	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	"git.redcoat.dev/cdn/pkg/provider/kubernetes"
)

type CloudFrontProvider struct {
	Clients map[string]cloudfront.CloudFront
}

func (p CloudFrontProvider) Has(status api.DistributionStatus) bool {
	return status.CloudFront.ID != ""
}

func (p CloudFrontProvider) Wants(class api.DistributionClassSpec) bool {
	return class.Providers.CloudFront != nil
}

func (p CloudFrontProvider) getSession(class api.DistributionClassSpec) *session.Session {
	config := aws.NewConfig()
	sessionOpts := session.Options{
		Config: *config,
	}
	sess, _ := session.NewSessionWithOptions(sessionOpts)

	return sess
}

// Creates a new CloudFront Provider from the given Distribution and
// calculated ResolvedOrigin
func (p CloudFrontProvider) Reconcile(
	class api.DistributionClassSpec,
	distro api.Distribution,
	origin kubernetes.ResolvedOrigin,
	cert *kubernetes.Certificate,
	status *api.DistributionStatus,
) error {
	sess := p.getSession(class)

	acm := CertificateProvider{
		// ACM certs must always be in us-east-1 for Cloudfront so we
		// override the region here
		Client: acm.New(sess, &aws.Config{
			Region: aws.String("us-east-1"),
		}),
		Certificate: cert,
		Status:      status,
	}

	err := acm.Reconcile()
	if err != nil {
		return err
	}

	cloudfront := DistributionProvider{
		Client:       cloudfront.New(sess),
		Distribution: distro,
		Class:        *class.Providers.CloudFront,
		Origin:       origin,
		Status:       status,
	}

	return cloudfront.Reconcile()
}

func (p CloudFrontProvider) Delete(
	class api.DistributionClassSpec,
	distro api.Distribution,
	status *api.DistributionStatus,
) error {
	sess := p.getSession(class)
	provider := DistributionProvider{
		Client:       cloudfront.New(sess),
		Distribution: distro,
		Status:       status,
	}

	return provider.Delete()
}
