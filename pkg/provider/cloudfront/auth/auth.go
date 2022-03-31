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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
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
