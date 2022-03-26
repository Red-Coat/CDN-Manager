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

	api "gitlab.com/redcoat/cdn-manager/pkg/api/v1alpha1"
	"gitlab.com/redcoat/cdn-manager/pkg/resolver"
)

type CloudFrontProvider struct {
	Clients map[string]session.Session
}

func (p CloudFrontProvider) Has(status api.DistributionStatus) bool {
	return status.CloudFront.ID != "" || status.CloudFront.CertificateArn != ""
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
	cert *resolver.Certificate,
	status *api.DistributionStatus,
) error {
	sess := p.getSession(class)

	err := NewCertificateProvider(sess, status, cert).Reconcile()
	if err != nil {
		return err
	}

	return NewDistributionProvider(sess, class, distro, status).
		Reconcile()
}

func (p CloudFrontProvider) Delete(
	class api.DistributionClassSpec,
	distro api.Distribution,
	status *api.DistributionStatus,
) error {
	sess := p.getSession(class)

	if status.CloudFront.ID != "" {
		err := NewDistributionProvider(sess, class, distro, status).Delete()
		if err != nil {
			return err
		}
	}

	// If the CloudFront distro has not been deleted yet, we can't attempt
	// to delete the certificate
	if status.CloudFront.ID != "" {
		return nil
	}

	return NewCertificateProvider(sess, status, nil).Delete()
}
