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
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"

	cfapi "git.redcoat.dev/cdn/pkg/api/provider/cloudfront"
	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	"git.redcoat.dev/cdn/pkg/provider/kubernetes"
)

type Provider struct {
	Client         *cloudfront.CloudFront
	Distribution   *api.Distribution
	Origin         kubernetes.ResolvedOrigin
	PreviousStatus *cfapi.CloudFrontStatus
	Endpoints      []api.Endpoint
}

// Creates a new CloudFront Provider from the given Distribution and
// calculated ResolvedOrigin
func NewProvider(distribution *api.Distribution, origin kubernetes.ResolvedOrigin) *Provider {
	config := aws.NewConfig()
	sessionOpts := session.Options{
		Config: *config,
	}
	sess, _ := session.NewSessionWithOptions(sessionOpts)
	client := cloudfront.New(sess)

	return &Provider{
		Client:         client,
		Distribution:   distribution,
		Origin:         origin,
		PreviousStatus: distribution.Status.CloudFront,
	}
}

// Returns the current CloudFront status
func (c *Provider) GetStatus() *cfapi.CloudFrontStatus {
	return c.Distribution.Status.CloudFront
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
func (c *Provider) CalculateMethods() ([]string, []string) {
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
func (c *Provider) CalculateViewerPolicy() string {
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
func (c *Provider) GenerateDistributionConfig() *cloudfront.DistributionConfig {
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
func (c *Provider) SetStatus(Distribution *cloudfront.Distribution) {
	c.Distribution.Status.CloudFront = &cfapi.CloudFrontStatus{
		State: *Distribution.Status,
		ID:    *Distribution.Id,
	}
	c.Endpoints = []api.Endpoint{api.Endpoint{
		Provider: "cloudfront",
		Host:     *Distribution.DomainName,
	}}
}

// Checks an existing Distribution's state matches with what is expected
// and updates it if not
func (c *Provider) Check() error {
	id := &c.GetStatus().ID

	current, err := c.Client.GetDistribution(&cloudfront.GetDistributionInput{
		Id: id,
	})
	c.SetStatus(current.Distribution)

	if awserr, ok := err.(awserr.RequestFailure); ok && awserr.StatusCode() == 404 {
		return c.Create()
	} else if err != nil {
		return err
	}

	desired := c.GenerateDistributionConfig()

	// If nothing has changed, we do not need to request an update
	if reflect.DeepEqual(desired, current.Distribution.DistributionConfig) {
		return nil
	}

	// We will not attempt any rewrites the status is in progress
	if *current.Distribution.Status == "InProgress" {
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
func (c *Provider) Create() error {
	info, err := c.Client.CreateDistribution(&cloudfront.CreateDistributionInput{
		DistributionConfig: c.GenerateDistributionConfig(),
	})

	c.SetStatus(info.Distribution)

	return err
}

func (c *Provider) Delete() {
	//
}

// Checks if the Status has been changed in this run
//
// This is used by the Distribution Controller to determine if it should
// regenerate the Distribution's status and save this back to the
// api-server.
func (c *Provider) IsDirty() bool {
	status := c.GetStatus()

	if status == nil {
		return false
	} else if c.PreviousStatus == nil {
		return true
	}

	return status.ID != c.PreviousStatus.ID || status.State != c.PreviousStatus.State
}

// Informs the Distribution Controller if this resource needs a recheck
//
// When a CloudFront Distribution is InProgress, we need to recheck it
// sooner than controller-runtime's usual refresh time, to check up on
// its update progress and, hopefully, update the Status to Deployed.
func (c *Provider) NeedsRecheck() bool {
	return c.GetStatus().State != "Deployed"
}

func (c *Provider) Has() bool {
	return c.GetStatus() != nil
}

func (c *Provider) Wants(spec api.DistributionClassSpec) bool {
	return spec.Providers.CloudFront != nil
}

func (c *Provider) GetEndpoints() []api.Endpoint {
	return c.Endpoints
}

// Checks if the CloudFront Distribution is ready
//
// This is used by the Distribution Controller to set the Distribution's
// overall "Ready" status field. If any Provider returns false, that
// field will be false.
func (c *Provider) IsReady() bool {
	return c.GetStatus().State == "Deployed"
}
