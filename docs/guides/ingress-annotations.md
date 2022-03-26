
# Ingress Annotations

In many cases, your application will be hosted within your cluster,
behind an ingress controller. **CDN Manager** watches for annotations on
`Ingress` records and will automatically setup appropriate
`Distribution` resources from them.

If you would like to use a `DistributionClass`, the annotation is:

```yaml
  cdn.redcoat.dev/distribution-class: distribution-class-name
```

If you would like to use a `ClusterDistributionClass`, the annotation
is:

```yaml
  cdn.redcoat.dev/cluster-distribution-class: cluster-distribution-class-name
```

## Behaviour

**CDN Manager** will create a `Distribution`, using the following
values:

- `distributionClass` - the `DistributionClass` or
`ClusterDistributionClass` named in the annotation.
- `origin` - CDN Manager will inspect the status of the `Ingress`
record, and copy across its `IngressLoadBalancer` hostname, if it has
one. If your ingress controller is working correctly, this should be
populated.
- `tls` - If the `Ingress` record is configured with TLS, CDN Manager
will enabled tls on the `Distribution` and use the same certificate
secret as the `Ingress`.

## Example

For the given `Ingress` record:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: example
  annotations:
    # This is the "magic" CDN line
    cdn.redcoat.dev/distribution-class: distribution-class-example
spec:
  rules:
      # This will be used as a hostname
    - host: example.com
      http:
        paths:
          - ...
      # This will be used as a hostname
    - host: example.net
      http:
        paths:
          - ...
  # If a TLS block is present, TLS will also be enabled on the
  # Distribution
  tls:
    - hosts:
        - example.com
        - example.net
      # This secret will be used to create a certificate in the CDN
      # provider
      secretName: myAwesomeSecret
# The status of the Ingress resource should be populated by your
# Ingress Controller, if it is working correctly.
status:
  loadBalancer:
    ingress:
      - hostname: nb-x-x-x-x.london.nodebalancer.linode.com
        ip: 8.8.8.8
```

This will result in the following `Distribution`:

```yaml
apiVersion: cdn.redcoat.dev/v1alpha1
kind: Distribution
metadata:
  # Copied from the ingress record
  name: example
spec:
  # Copied from annotation name
  distributionClass:
    kind: DistributionClass
    name: distribution-class-example

  # Derived rom the Ingress rules
  hosts:
    - example.com
    - example.net

  origin:
    # CDN manager will prefer to use hostnames rather than IP addresses,
    # because AWS CloudFront only supports hostname origins.
    hostname: nb-x-x-x-x.london.nodebalancer.linode.com

    # There is not currently a way to change these for Distributions
    # created via Ingress annotations.
    # If you need to use non-standard ports, you may need to create the
    # Distribution resource rather than using the Ingress annotation
    # method.
    httpPort: 80
    httpsPort: 443

  tls:
    secretName: myAwesomeSecret
```
