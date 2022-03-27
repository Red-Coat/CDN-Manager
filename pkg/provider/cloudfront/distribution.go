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
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/service/cloudfront"

	api "gitlab.com/redcoat/cdn-manager/pkg/api/v1alpha1"
	cfapi "gitlab.com/redcoat/cdn-manager/pkg/provider/cloudfront/api/v1alpha1"
)

type DistributionProvider struct {
	Client       *cloudfront.CloudFront
	Distribution api.Distribution
	Class        cfapi.CloudFrontSpec
	Status       *api.DistributionStatus
	CurrentState *cloudfront.Distribution
	DesiredState *cloudfront.DistributionConfig
}

// Sets up a new instance of the DistributionProvider
func NewDistributionProvider(
	cfg client.ConfigProvider,
	class api.DistributionClassSpec,
	distro api.Distribution,
	status *api.DistributionStatus,
) *DistributionProvider {
	provider := DistributionProvider{
		Client:       cloudfront.New(cfg),
		Class:        *class.Providers.CloudFront,
		Distribution: distro,
		Status:       status,
	}

	return &provider
}

// Calculates the Allowed and Cached methods for the CloudFront
// Distribution
//
// CloudFront only supports limited subsets:
// - HEAD and GET only
// - HEAD, GET and OPTIONS
// - HEAD, GET, OPTIONS, POST, PUT, and DELETE
//
// HEAD and GET requests are always cached. POST, PUT, and DELETE are
// never cached. OPTIONS can optionally be cached (this method will
// always cache OPTIONS, if it is set)
func (c *DistributionProvider) calculateMethods() ([]string, []string) {
	methods := []string{"HEAD", "GET"}
	for _, header := range c.Class.SupportedMethods {
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

func (c *DistributionProvider) calculateViewerCertificate() *cloudfront.ViewerCertificate {
	var cert cloudfront.ViewerCertificate
	if c.Status.CloudFront.CertificateArn != "" {
		arn := aws.String(c.Status.CloudFront.CertificateArn)
		cert.ACMCertificateArn = arn
		cert.Certificate = arn
		cert.
			SetCloudFrontDefaultCertificate(false).
			SetCertificateSource("acm").
			SetMinimumProtocolVersion("TLSv1.2_2021").
			SetSSLSupportMethod(c.Class.SSLMode)
	} else {
		cert.
			SetCertificateSource("cloudfront").
			SetCloudFrontDefaultCertificate(true).
			SetMinimumProtocolVersion("TLSv1")
	}

	return &cert
}

func (c *DistributionProvider) calculateAliases() *cloudfront.Aliases {
	aliases := cloudfront.Aliases{
		Quantity: aws.Int64(int64(len(c.Distribution.Spec.Hosts))),
	}
	if *aliases.Quantity > 0 {
		aliases.Items = aws.StringSlice(c.Distribution.Spec.Hosts)
	}

	return &aliases
}

// Calculates the desired forwarded values for the distribution
func (c *DistributionProvider) calculateForwardedValues() *cloudfront.ForwardedValues {
	// If a cache policy id is set then this takes precendence. We will
	// hope that it has been setup appropriately to forward the host
	// header.
	if c.Class.CachePolicyId != "" {
		return nil
	}

	return &cloudfront.ForwardedValues{
		// Best guess defaults - if you want to change these, set a cache
		// policy / origin request policy
		Cookies: &cloudfront.CookiePreference{
			Forward: aws.String(cloudfront.ItemSelectionNone),
		},
		QueryString: aws.Bool(true),
		QueryStringCacheKeys: &cloudfront.QueryStringCacheKeys{
			Quantity: aws.Int64(0),
		},
		Headers: &cloudfront.Headers{
			Quantity: aws.Int64(1),
			// We _have_ to forward the Host header so that the ingress
			// controller knows what to do with it
			Items: aws.StringSlice([]string{"Host"}),
		},
	}
}

// Calculates the TTLs to set on the distribution
//
// If a Cache Policy Id has been set, this will just return nils. If
// not, it will return the AWS default TTLs.
func (c *DistributionProvider) calculateTTLs() (*int64, *int64, *int64) {
	// If a cache policy id is set, we don't need to set the TTLs on the
	// distribution itself, so we'll just return nil
	if c.Class.CachePolicyId != "" {
		return nil, nil, nil
	}

	// AWS' default TTL settings
	return aws.Int64(0), aws.Int64(31536000), aws.Int64(86400)
}

// Calculates the full desired state of the CloudFront Distribution
//
// This is used to create new Distributions, to compare against existing
// Distributions, and to update Distributions if their state does not
// match.
func (c *DistributionProvider) generateDistributionConfig(enabled bool) {
	supportedMethods, cachedMethods := c.calculateMethods()
	minTTL, maxTTL, defaultTTL := c.calculateTTLs()

	c.DesiredState = &cloudfront.DistributionConfig{
		CallerReference: aws.String(string(c.Distribution.UID)),
		Comment:         aws.String("Managed By cdn.redcoat.dev"),
		Enabled:         aws.Bool(enabled),
		IsIPV6Enabled:   aws.Bool(true),
		Origins: &cloudfront.Origins{
			Quantity: aws.Int64(1),
			Items: []*cloudfront.Origin{&cloudfront.Origin{
				DomainName:         aws.String(c.Distribution.Spec.Origin.Host),
				Id:                 aws.String(c.Distribution.Spec.Origin.Host),
				ConnectionAttempts: aws.Int64(3),
				ConnectionTimeout:  aws.Int64(10),
				CustomHeaders: &cloudfront.CustomHeaders{
					Quantity: aws.Int64(0),
				},
				OriginPath: aws.String(""),
				CustomOriginConfig: &cloudfront.CustomOriginConfig{
					HTTPPort:               aws.Int64(int64(c.Distribution.Spec.Origin.HTTPPort)),
					HTTPSPort:              aws.Int64(int64(c.Distribution.Spec.Origin.HTTPSPort)),
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
		Aliases: c.calculateAliases(),
		CacheBehaviors: &cloudfront.CacheBehaviors{
			Quantity: aws.Int64(0),
		},
		Restrictions: &cloudfront.Restrictions{
			GeoRestriction: &cloudfront.GeoRestriction{
				Quantity:        aws.Int64(0),
				RestrictionType: aws.String(cloudfront.GeoRestrictionTypeNone),
			},
		},
		ViewerCertificate: c.calculateViewerCertificate(),
		PriceClass:        aws.String(cloudfront.PriceClassPriceClassAll),
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
			TargetOriginId:        aws.String(c.Distribution.Spec.Origin.Host),
			ViewerProtocolPolicy:  aws.String(c.calculateViewerPolicy()),
			Compress:              aws.Bool(true),
			CachePolicyId:         stringOrNil(c.Class.CachePolicyId),
			OriginRequestPolicyId: stringOrNil(c.Class.OriginRequestPolicyId),
			ForwardedValues:       c.calculateForwardedValues(),
			MinTTL:                minTTL,
			MaxTTL:                maxTTL,
			DefaultTTL:            defaultTTL,
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
	c.Status.CloudFront.State = *state.Status
	c.Status.CloudFront.ID = *state.Id
	c.removeCloudFrontEndpoints()
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
		c.Status.CloudFront.ID = ""
		c.removeCloudFrontEndpoints()
		return nil, nil
	} else if err != nil {
		return nil, err
	} else {
		c.CurrentState = res.Distribution
		c.setStatus()
		return res.ETag, nil
	}
}

func (c *DistributionProvider) update(etag *string) (*string, error) {
	res, err := c.Client.UpdateDistribution(&cloudfront.UpdateDistributionInput{
		DistributionConfig: c.DesiredState,
		Id:                 c.CurrentState.Id,
		IfMatch:            etag,
	})

	if err != nil {
		return nil, err
	} else {
		c.CurrentState = res.Distribution
		c.setStatus()
		return res.ETag, nil
	}
}

func (c *DistributionProvider) Reconcile() error {
	if c.Distribution.Status.CloudFront.ID != "" {
		return c.Check()
	} else {
		return c.Create()
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
			c.Status.CloudFront.ID = re.FindString(awserr.Message())
			c.Status.CloudFront.State = "Unknown"
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
		c.Status.CloudFront.ID = ""
		c.removeCloudFrontEndpoints()

		return nil
	}
}

// stringOrNil checks to see if a string has any value - if it does, it
// returns a pointer to the string. If it doesn't (ie it is an empty
// string), it returns nil.
//
// This is useful for AWS' weird-ass everything-is-a-pointer SDK.
func stringOrNil(value string) *string {
	if value == "" {
		return nil
	} else {
		return &value
	}
}
