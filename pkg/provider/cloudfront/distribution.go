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
	"regexp"

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
	CurrentState *cloudfront.Distribution
	DesiredState *cloudfront.DistributionConfig
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
func (c *DistributionProvider) calculateMethods() ([]string, []string) {
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
func (c *DistributionProvider) calculateViewerPolicy() string {
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
func (c *DistributionProvider) generateDistributionConfig(enabled bool) {
	supportedMethods, cachedMethods := c.calculateMethods()

	c.DesiredState = &cloudfront.DistributionConfig{
		CallerReference: aws.String(string(c.Distribution.UID)),
		Comment:         aws.String("Managed By cdn.redcoat.dev"),
		Enabled:         aws.Bool(enabled),
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
			ViewerProtocolPolicy:  aws.String(c.calculateViewerPolicy()),
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
func (c *DistributionProvider) setStatus() {
	state := c.CurrentState
	c.Status.CloudFront = &cfapi.CloudFrontStatus{
		State: *state.Status,
		ID:    *state.Id,
	}
	c.Status.Endpoints = append(c.Status.Endpoints, api.Endpoint{
		Provider: "cloudfront",
		Host:     *state.DomainName,
	})
	c.Status.Ready = c.Status.Ready && *state.Status == "Deployed"
}

func isAwsError(err error, code string) (bool, awserr.Error) {
	if awserr, ok := err.(awserr.Error); ok {
		if awserr.Code() == code {
			return true, awserr
		} else {
			return false, awserr
		}
	}

	return false, nil
}

func (c *DistributionProvider) load() (*string, error) {
	res, err := c.Client.GetDistribution(&cloudfront.GetDistributionInput{
		Id: &c.Distribution.Status.CloudFront.ID,
	})

	if is, _ := isAwsError(err, "NoSuchDistribution"); is {
		return nil, nil
	} else if err != nil {
		c.useLastKnownStatus()
		return nil, err
	} else {
		c.CurrentState = res.Distribution
		c.setStatus()
		return res.ETag, nil
	}
}

func (c *DistributionProvider) useLastKnownStatus() {
	c.Status.CloudFront = c.Distribution.Status.CloudFront
	for _, endpoint := range c.Distribution.Status.Endpoints {
		if endpoint.Provider == "cloudfront" {
			c.Status.Endpoints = append(c.Status.Endpoints, endpoint)
			break
		}
	}
}

func (c *DistributionProvider) update(etag *string) (*string, error) {
	res, err := c.Client.UpdateDistribution(&cloudfront.UpdateDistributionInput{
		DistributionConfig: c.DesiredState,
		Id:                 c.CurrentState.Id,
		IfMatch:            etag,
	})

	if err != nil {
		c.useLastKnownStatus()
		return nil, err
	} else {
		c.CurrentState = res.Distribution
		c.setStatus()
		return res.ETag, nil
	}
}

// Checks an existing Distribution's state matches with what is expected
// and updates it if not
func (c *DistributionProvider) Check() error {
	etag, err := c.load()

	if err != nil {
		return err
	}

	if etag == nil {
		return c.Create()
	}

	c.generateDistributionConfig(true)

	// If nothing has changed, we do not need to request an update
	if reflect.DeepEqual(c.DesiredState, c.CurrentState.DistributionConfig) {
		return nil
	}

	_, err = c.update(etag)
	return err
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
	c.generateDistributionConfig(true)
	current, err := c.Client.CreateDistribution(&cloudfront.CreateDistributionInput{
		DistributionConfig: c.DesiredState,
	})

	if err != nil {
		// In the case that we get DistributionAlreadyExists, that indicates
		// that the caller reference has already been taken.
		// The caller reference is the k8s resource UID so it is very
		// likely that this is the distribution that we want, but the status
		// has been lost somehow.
		// We'll grab the distribution id in question out of the error
		// message and assume it is ours.
		// We won't do any further reconcilliation at this time, as it is
		// worth having this request requeued so that rate limiting can
		// occur.
		if is, awserr := isAwsError(err, "DistributionAlreadyExists"); is {
			re := regexp.MustCompile(`[A-Z0-9]{14}`)
			c.Status.CloudFront = &cfapi.CloudFrontStatus{
				ID:    re.FindString(awserr.Message()),
				State: "Unknown",
			}
		}

		return err
	}

	c.CurrentState = current.Distribution
	c.setStatus()

	return nil
}

func (c *DistributionProvider) removeCloudFrontEndpoints() {
	var endpoints []api.Endpoint
	for _, endpoint := range c.Status.Endpoints {
		if endpoint.Provider != "cloudfront" {
			endpoints = append(endpoints, endpoint)
		}
	}

	c.Status.Endpoints = endpoints
}

func (c *DistributionProvider) Delete() error {
	etag, err := c.load()
	if err != nil {
		return err
	} else if etag == nil {
		// If the distribution didn't exist, we don't need to do anything
		return nil
	}

	if *c.CurrentState.DistributionConfig.Enabled {
		// Bit of a nasty hack
		c.DesiredState = c.CurrentState.DistributionConfig
		c.DesiredState.SetEnabled(false)
		_, err = c.update(etag)
		return err
	}

	// We have to wait until the distribution is completely disabled
	// before instructing AWS to delete it
	if *c.CurrentState.Status == "InProgress" {
		return nil
	}

	// We ignore the DeleteDistributionOutput because it doesn't contain
	// anything
	_, err = c.Client.DeleteDistribution(&cloudfront.DeleteDistributionInput{
		Id:      c.CurrentState.Id,
		IfMatch: etag,
	})

	if err != nil {
		return err
	} else {
		c.Status.CloudFront = nil
		c.removeCloudFrontEndpoints()

		return nil
	}
}
