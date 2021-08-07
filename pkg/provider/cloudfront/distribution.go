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
	"reflect"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudfront"

	cfapi "git.redcoat.dev/cdn/pkg/api/provider/cloudfront"
	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	"git.redcoat.dev/cdn/pkg/provider/kubernetes"
)

type DistributionProvider struct {
	Client       *cloudfront.CloudFront
	Distribution api.Distribution
	Origin       kubernetes.ResolvedOrigin
	Status       *api.DistributionStatus
}

// Calculates the Allowed and Cached methods for the CloudFront
// Distribution
//
// The k8s Distribution tries to be relatively generic in the way it is
// defined, so you can specify any list of supported HTTP Methods,
// however CloudFront only supports limited subsets:
// - HEAD and GET only
// - HEAD, GET and OPTIONS
// - HEAD, GET, OPTIONS, POST, PUT, and DELETE
//
// HEAD and GET requests are always cached. POST, PUT, and DELETE are
// never cached. OPTIONS can optionally be cached (this method will
// always cache OPTIONS, if it is set)
func (c *DistributionProvider) CalculateMethods() ([]string, []string) {
	methods := []string{"HEAD", "GET"}
	for _, header := range c.Distribution.Spec.SupportedMethods {
		if header == "OPTIONS" {
			methods = append(methods, "OPTIONS")
		} else if header == "POST" || header == "PUT" || header == "DELETE" {
			return []string{"HEAD", "GET", "OPTIONS", "POST", "PUT", "DELETE"},
				[]string{"HEAD", "GET", "OPTIONS"}
		}
	}

	return methods, methods
}

// Calculates the CloudFront TLS ViewerPolicy from the Distribution's
// TLS Settings
func (c *DistributionProvider) CalculateViewerPolicy() string {
	tls := c.Distribution.Spec.TLS
	if tls == nil || tls.Mode == "both" {
		return cloudfront.ViewerProtocolPolicyAllowAll
	}

	if tls.Mode == "only" {
		return cloudfront.ViewerProtocolPolicyHttpsOnly
	}

	return cloudfront.ViewerProtocolPolicyRedirectToHttps
}

// Calculates the full desired state of the CloudFront Distribution
//
// This is used to create new Distributions, to compare against existing
// Distributions, and to update Distributions if their state does not
// match.
func (c *DistributionProvider) GenerateDistributionConfig() *cloudfront.DistributionConfig {
	supportedMethods, cachedMethods := c.CalculateMethods()

	return &cloudfront.DistributionConfig{
		CallerReference: aws.String(string(c.Distribution.UID)),
		Comment:         aws.String("Managed By cdn.redcoat.dev"),
		Enabled:         aws.Bool(true),
		IsIPV6Enabled:   aws.Bool(true),
		Origins: &cloudfront.Origins{
			Quantity: aws.Int64(1),
			Items: []*cloudfront.Origin{&cloudfront.Origin{
				DomainName:         aws.String(c.Origin.Host),
				Id:                 aws.String(c.Origin.Host),
				ConnectionAttempts: aws.Int64(3),
				ConnectionTimeout:  aws.Int64(10),
				CustomHeaders: &cloudfront.CustomHeaders{
					Quantity: aws.Int64(0),
				},
				OriginPath: aws.String(""),
				CustomOriginConfig: &cloudfront.CustomOriginConfig{
					HTTPPort:               aws.Int64(int64(c.Origin.HTTPPort)),
					HTTPSPort:              aws.Int64(int64(c.Origin.HTTPSPort)),
					OriginProtocolPolicy:   aws.String("match-viewer"),
					OriginReadTimeout:      aws.Int64(30),
					OriginKeepaliveTimeout: aws.Int64(30),
					OriginSslProtocols: &cloudfront.OriginSslProtocols{
						Quantity: aws.Int64(1),
						Items:    aws.StringSlice([]string{"TLSv1.2"}),
					},
				},
			}},
		},
		CustomErrorResponses: &cloudfront.CustomErrorResponses{
			Quantity: aws.Int64(0),
		},
		OriginGroups: &cloudfront.OriginGroups{
			Quantity: aws.Int64(0),
		},
		Aliases: &cloudfront.Aliases{
			Quantity: aws.Int64(int64(len(c.Distribution.Spec.Hosts))),
			//Items:    aws.StringSlice(c.Distribution.Spec.Hosts),
		},
		CacheBehaviors: &cloudfront.CacheBehaviors{
			Quantity: aws.Int64(0),
		},
		Restrictions: &cloudfront.Restrictions{
			GeoRestriction: &cloudfront.GeoRestriction{
				Quantity:        aws.Int64(0),
				RestrictionType: aws.String(cloudfront.GeoRestrictionTypeNone),
			},
		},
		ViewerCertificate: &cloudfront.ViewerCertificate{
			CertificateSource:            aws.String("cloudfront"),
			CloudFrontDefaultCertificate: aws.Bool(true),
			MinimumProtocolVersion:       aws.String("TLSv1"),
		},
		PriceClass: aws.String(cloudfront.PriceClassPriceClassAll),
		Logging: &cloudfront.LoggingConfig{
			Enabled:        aws.Bool(false),
			Bucket:         aws.String(""),
			IncludeCookies: aws.Bool(false),
			Prefix:         aws.String(""),
		},
		DefaultRootObject: aws.String(""),
		WebACLId:          aws.String(""),
		HttpVersion:       aws.String("http2"),
		DefaultCacheBehavior: &cloudfront.DefaultCacheBehavior{
			TargetOriginId:        aws.String(c.Origin.Host),
			ViewerProtocolPolicy:  aws.String(c.CalculateViewerPolicy()),
			Compress:              aws.Bool(true),
			CachePolicyId:         aws.String("658327ea-f89d-4fab-a63d-7e88639e58f6"),
			OriginRequestPolicyId: aws.String("216adef6-5c7f-47e4-b989-5492eafa07d3"),
			// Required By AWS
			SmoothStreaming:        aws.Bool(false),
			FieldLevelEncryptionId: aws.String(""),
			TrustedSigners: &cloudfront.TrustedSigners{
				Enabled:  aws.Bool(false),
				Quantity: aws.Int64(0),
			},
			LambdaFunctionAssociations: &cloudfront.LambdaFunctionAssociations{
				Quantity: aws.Int64(0),
			},
			AllowedMethods: &cloudfront.AllowedMethods{
				Quantity: aws.Int64(int64(len(supportedMethods))),
				Items:    aws.StringSlice(supportedMethods),
				CachedMethods: &cloudfront.CachedMethods{
					Quantity: aws.Int64(int64(len(cachedMethods))),
					Items:    aws.StringSlice(cachedMethods),
				},
			},
		},
	}
}

// Sets the Status based on the Status returned by the AWS API
func (c *DistributionProvider) SetStatus(Distribution *cloudfront.Distribution) {
	c.Status.CloudFront = &cfapi.CloudFrontStatus{
		State: *Distribution.Status,
		ID:    *Distribution.Id,
	}
	c.Status.Endpoints = append(c.Status.Endpoints, api.Endpoint{
		Provider: "cloudfront",
		Host:     *Distribution.DomainName,
	})
	c.Status.Ready = c.Status.Ready && c.Status.CloudFront.State == "Deployed"
}

// Checks an existing Distribution's state matches with what is expected
// and updates it if not
func (c *DistributionProvider) Check() error {
	id := &c.Distribution.Status.CloudFront.ID

	current, err := c.Client.GetDistribution(&cloudfront.GetDistributionInput{
		Id: id,
	})

	if awserr, ok := err.(awserr.RequestFailure); ok && awserr.StatusCode() != 404 {
		return c.Create()
	} else if err != nil {
		c.Status.CloudFront = c.Distribution.Status.CloudFront
		for _, endpoint := range c.Distribution.Status.Endpoints {
			if endpoint.Provider == "cloudfront" {
				c.Status.Endpoints = append(c.Status.Endpoints, endpoint)
				break
			}
		}
		return err
	}

	c.SetStatus(current.Distribution)
	desired := c.GenerateDistributionConfig()

	// If nothing has changed, we do not need to request an update
	if reflect.DeepEqual(desired, current.Distribution.DistributionConfig) {
		return nil
	}

	updated, err := c.Client.UpdateDistribution(&cloudfront.UpdateDistributionInput{
		DistributionConfig: desired,
		Id:                 id,
		IfMatch:            current.ETag,
	})

	if err != nil {
		return err
	}

	c.SetStatus(updated.Distribution)

	return nil
}

// Creates a CloudFront Distribution and sets its status on the
// Distribution resource
//
// This is called in one of two circumstances:
// - The Distribution Controller does not find any CloudFront state on
//   the Distribution.
// - The Distribution Controller has found CloudFront state, but when
//   Check() was running, AWS returned a Not Found on it (implying the
//   Distribution has been destroyed).
func (c *DistributionProvider) Create() error {
	info, err := c.Client.CreateDistribution(&cloudfront.CreateDistributionInput{
		DistributionConfig: c.GenerateDistributionConfig(),
	})

	if err != nil {
		return err
	}

	c.SetStatus(info.Distribution)

	return nil
}
