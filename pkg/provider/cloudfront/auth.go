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
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	authv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1rest "k8s.io/client-go/kubernetes/typed/core/v1"

	cfapi "gitlab.com/redcoat/cdn-manager/pkg/provider/cloudfront/api/v1alpha1"
)

type AwsAuthProvider struct {
	session     *session.Session
	stsApi      *sts.STS
	sessionName string
	corev1      *corev1rest.CoreV1Interface
}

func NewAwsAuthProvider(sessionName string, corev1 *corev1rest.CoreV1Interface) (*AwsAuthProvider, error) {
	sess, err := newSession()
	if err != nil {
		return nil, err
	}

	return &AwsAuthProvider{
		session:     sess,
		stsApi:      sts.New(sess),
		sessionName: sessionName,
		corev1:      corev1,
	}, nil
}

func newSession() (*session.Session, error) {
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}

	return sess, nil
}

func (p *AwsAuthProvider) NewSession(details *cfapi.AwsAuth, namespace *string) (*session.Session, error) {
	if details == nil {
		return p.session, nil
	}

	if details.JWTAuth != nil {
		creds, err := p.credentialsForJwtAuth(context.TODO(), details.JWTAuth, namespace)
		if err != nil {
			return nil, err
		}

		config := aws.NewConfig()
		config.WithCredentials(creds)

		return session.NewSession(config)
	}

	return p.session, nil
}

func (p *AwsAuthProvider) credentialsForJwtAuth(
	ctx context.Context,
	details *cfapi.AwsJwtAuth,
	namespace *string,
) (*credentials.Credentials, error) {
	if namespace == nil {
		if namespace = details.ServiceAccount.Namespace; namespace == nil {
			return nil, fmt.Errorf("Service Account had no namespace (required for cluster-scoped resources)")
		}
	}

	name := details.ServiceAccount.Name
	saApi := (*p.corev1).ServiceAccounts(*namespace)
	serviceAccount, err := saApi.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	roleArn := serviceAccount.Annotations[details.AnnotationName]
	if roleArn == "" {
		return nil, fmt.Errorf("No role was annotated on the given Service Account")
	}

	return credentials.NewCredentials(stscreds.NewWebIdentityRoleProviderWithToken(
		p.stsApi,
		roleArn,
		p.sessionName,
		tokenFetcher{
			api:            saApi,
			serviceAccount: name,
			aud:            details.Audience,
		},
	)), nil
}

type tokenFetcher struct {
	api            corev1rest.ServiceAccountInterface
	serviceAccount string
	aud            string
}

func (f tokenFetcher) FetchToken(ctx credentials.Context) ([]byte, error) {
	tokenResponse, err := f.api.CreateToken(ctx, f.serviceAccount, &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences: []string{f.aud},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return []byte(tokenResponse.Status.Token), nil
}
