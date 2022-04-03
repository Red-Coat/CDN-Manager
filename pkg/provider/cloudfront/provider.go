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
	corev1rest "k8s.io/client-go/kubernetes/typed/core/v1"

	api "gitlab.com/redcoat/cdn-manager/pkg/api/v1alpha1"
	"gitlab.com/redcoat/cdn-manager/pkg/provider/cloudfront/auth"
	"gitlab.com/redcoat/cdn-manager/pkg/resolver"
)

type CloudFrontProvider struct {
	Auth *auth.AwsAuthProvider
}

func New(corev1 corev1rest.CoreV1Interface) (*CloudFrontProvider, error) {
	auth, err := auth.NewAwsAuthProvider("cdn-manager", &corev1)
	if err != nil {
		return nil, err
	}

	return &CloudFrontProvider{
		Auth: auth,
	}, nil
}

func (p CloudFrontProvider) Wants(class api.DistributionClassSpec) bool {
	return class.Providers.CloudFront != nil
}

// Creates a new CloudFront Provider from the given Distribution and
// calculated ResolvedOrigin
func (p CloudFrontProvider) Reconcile(
	class api.DistributionClassSpec,
	distro api.Distribution,
	cert *resolver.Certificate,
	status *api.DistributionStatus,
) error {
	sess, _ := p.Auth.NewSession(class.Providers.CloudFront.Auth, nil)

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
	sess, _ := p.Auth.NewSession(class.Providers.CloudFront.Auth, nil)

	if status.ExternalId != "" {
		err := NewDistributionProvider(sess, class, distro, status).Delete()
		if err != nil {
			return err
		}
	}

	// If the CloudFront distro has not been deleted yet, we can't attempt
	// to delete the certificate
	if status.ExternalId != "" {
		return nil
	}

	return NewCertificateProvider(sess, status, nil).Delete()
}
