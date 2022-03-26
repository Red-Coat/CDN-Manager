# Distribution

A `Distribution` resource represents a service which will be setup /
synced by an external Content Delivery Network. It describes the host
name(s) that the CDN should respond to, as well as origin address that
the CDN's cache should request resources from.

## Example

```yaml
apiVersion: cdn.redcoat.dev/v1alpha1
kind: Distribution
metadata:
  name: distribution-example
spec:
  # Which CDN provider should we use
  # See the DistributionClass / ClusterDistributionClass docs for
  # further details about these resources
  distributionClass:
    kind: DistributionClass # Or ClusterDistributionClass
    name: distribution-class-name

  # The hostnames the CDN will serve traffic from
  hosts:
    - example.com
    - example.net

  # Details about the "origin": the source server / domain which is
  # being cached.
  origin:
    # The hostname or IP address of the origin
    # For typical kubernetes setups, this will often be the hostname of
    # the cluster's ingress cloud load balancer.
    # NB: AWS CloudFront does not support IP address origin hosts.
    # Required.
    host: nb-x-x-x-x.london.nodebalancer.linode.com

    # The port the origin uses for HTTP requests
    # Optional. Default is 80
    httpPort: 80

    # The port the origin uses for HTTPS requests
    # Optional. Default is 443
    httpsPort: 443

  # Optional configuration about how the CDN should handle HTTPS traffic
  tls:
    # Mode can be one of:
    #   Redirect - HTTP requests are permanently redirected to HTTPS
    #   Only - Only HTTPS requests are listened to. HTTP is ignored.
    #   Both - The CDN will respond to traffic on both HTTP and HTTPS
    # Optional. Default is Redirect if TLS configuration is present.
    mode: Redirect

    # The name of the kubernetes secret holding the TLS certificate the
    # CDN should use to serve traffic.
    # This must be of type kubernetes.io/tls.
    # Required.
    secretName: my-tls-cert
```
