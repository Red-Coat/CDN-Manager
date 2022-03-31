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

package auth

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws/credentials"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cfapi "gitlab.com/redcoat/cdn-manager/pkg/provider/cloudfront/api/v1alpha1"
)

// Loads static credentials from a secret
func (p *AwsAuthProvider) credentialsForAccessKey(
	ctx context.Context,
	details *cfapi.NamespacedName,
	namespace *string,
) (*credentials.Credentials, error) {
	if namespace == nil {
		if namespace = details.Namespace; namespace == nil {
			return nil, fmt.Errorf("Secret had no namespace (required for cluster-scoped resources)")
		}
	}

	secretsApi := (*p.corev1).Secrets(*namespace)
	secret, err := secretsApi.Get(ctx, details.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	accessKey := string(secret.Data["AWS_ACCESS_KEY_ID"])
  secretKey := string(secret.Data["AWS_SECRET_ACCESS_KEY"])
	if accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("Secret missing the AWS Key")
	}

	return credentials.NewStaticCredentials(accessKey, secretKey, ""), nil
}
