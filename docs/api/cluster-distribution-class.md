# ClusterDistributionClass

`ClusterDistributionClass` resources are cluster-scoped versions of a
`DistributionClass` - a representations of a specific CDN provider. This
includes details of which CDN to use (eg AWS CloudFront), how to
authenticate with it, and any provider-specific configurations or
behaviours for the provider.

## Example

```yaml
apiVersion: cdn.redcoat.dev/v1alpha1
kind: ClusterDistributionClass
metadata:
  name: cluster-distribution-class-name
spec:
  # Details of which provider to use
  providers:
    # Specify this block to cause Distribution resources to be synced to
    # AWS CloudFront.
    cloudfront:
      
      # If you have previously created a CloudFront Cache or Origin
      # Policy, you can specify their IDs here. Any created
      # distributions will use these policies. If not specified, the
      # distributions will be created in "Legacy" mode with sensible
      # defaults (Host header, cookies and query params will be cached
      # on and sent to the origin).
      cachePolicyId: 658327ea-f89d-4fab-a63d-7e88639e58f6
      originRequestPolicyId: 658327ea-f89d-4fab-a63d-7e88639e58f6

      # Normally, CloudFront serves traffic using SNI, which allows them
      # to serve many customers using the same IP addresses. If your
      # application has specific requirements where SNI will not work,
      # you can configure static or virtual IPs here, but beware these
      # incur very high charges. This feature is not normally required
      # unless your clients are legacy and do not support SNI.
      # Acceptable values:
      #   sni-only - Serve traffic using sni
      #   vpi - Serve traffic using a virtual ip
      #   static-ip - Serve traffic using a static ip
      # Optional. Default is "sni-only". Consult AWS Documentation /
      # Customer service if you want to change this.
      sslMode: sni-only

      # List of HTTP methods to support
      supportedMethods:
        - GET
        - HEAD
```
