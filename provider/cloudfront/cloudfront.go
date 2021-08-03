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

	api "redcoat.dev/cdn/api/v1alpha1"
	"redcoat.dev/cdn/provider/kubernetes"
)

type Provider struct {
	client         *cloudfront.CloudFront
	distribution   *api.Distribution
	Origin         kubernetes.ResolvedOrigin
	PreviousStatus *api.CloudFrontStatus
	Endpoints      []api.Endpoint
}

func NewProvider(distribution *api.Distribution, origin kubernetes.ResolvedOrigin) (*Provider, error) {
	config := aws.NewConfig()
	sessionOpts := session.Options{
		Config: *config,
	}
	sess, _ := session.NewSessionWithOptions(sessionOpts)
	client := cloudfront.New(sess)

	return &Provider{
		client:         client,
		distribution:   distribution,
		Origin:         origin,
		PreviousStatus: distribution.Status.CloudFront,
	}, nil
}

func (c *Provider) GetStatus() *api.CloudFrontStatus {
	return c.distribution.Status.CloudFront
}

func (c *Provider) CalculateMethods() ([]string, []string) {
	methods := []string{"HEAD", "GET"}
	cached := []string{"HEAD", "GET"}
	for _, header := range c.distribution.Spec.SupportedMethods {
		if header == "OPTIONS" {
			methods = append(methods, "OPTIONS")
			cached = append(cached, "OPTIONS")
		} else if header == "POST" || header == "PUT" || header == "DELETE" {
			if len(methods) == 2 {
				methods = append(methods, "OPTIONS")
				cached = append(cached, "OPTIONS")
			}
			methods = append(methods, "POST", "PUT", "DELETE")
			break
		}
	}

	return methods, cached
}

func (c *Provider) CalculateViewerPolicy() string {
	tls := c.distribution.Spec.TLS
	if tls == nil || tls.Mode == "both" {
		return cloudfront.ViewerProtocolPolicyAllowAll
	}

	if tls.Mode == "only" {
		return cloudfront.ViewerProtocolPolicyHttpsOnly
	}

	return cloudfront.ViewerProtocolPolicyRedirectToHttps
}

func (c *Provider) GenerateDistributionConfig() *cloudfront.DistributionConfig {
	supportedMethods, cachedMethods := c.CalculateMethods()

	return &cloudfront.DistributionConfig{
		CallerReference: aws.String(string(c.distribution.UID)),
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
			Quantity: aws.Int64(int64(len(c.distribution.Spec.Hosts))),
			//Items:    aws.StringSlice(c.distribution.Spec.Hosts),
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

func (c *Provider) CalculateStatus(status string) string {
	if status == "Deployed" {
		return "Redeployed"
	} else {
		return ""
	}
}

func (c *Provider) SetStatus(Distribution *cloudfront.Distribution) {
	c.distribution.Status.CloudFront = &api.CloudFrontStatus{
		State: *Distribution.Status,
		ID:    *Distribution.Id,
	}
	c.Endpoints = []api.Endpoint{api.Endpoint{
		Provider: "cloudfront",
		Host:     *Distribution.DomainName,
	}}
}

func (c *Provider) Check() error {
	id := &c.GetStatus().ID

	current, err := c.client.GetDistribution(&cloudfront.GetDistributionInput{
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

	updated, err := c.client.UpdateDistribution(&cloudfront.UpdateDistributionInput{
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

func (c *Provider) Create() error {
	info, err := c.client.CreateDistribution(&cloudfront.CreateDistributionInput{
		DistributionConfig: c.GenerateDistributionConfig(),
	})

	if err != nil {
		return err
	}

	c.SetStatus(info.Distribution)

	return nil
}

func (c *Provider) Delete() {
	//
}

func (c *Provider) IsDirty() bool {
	status := c.GetStatus()

	if status == nil {
		return false
	} else if c.PreviousStatus == nil {
		return true
	}

	return status.ID != c.PreviousStatus.ID || status.State != c.PreviousStatus.State
}

func (c *Provider) NeedsRecheck() bool {
	return c.GetStatus().State != "Deployed"
}

func (c *Provider) IsReady() bool {
	return c.GetStatus().State == "Deployed"
}
