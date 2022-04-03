# CDN Manager

CDN Manager lets you manage your CDN Distributions via Kubernetes
resources, and syncs ingresses, load balancers and certificate settings
automatically for you.

It currently only support [AWS CloudFront][2], however support for
CloudFlare and Fastly is on the roadmap.

Its technical design has drawn some inspiration from [cert-manager][1].

**This is still in alpha stage - not all features may be supported, and
things may change without notice.**

## Installation

Via helm:

```
helm repo add redcoat https://charts.redcoat.dev
helm install cdn-manager redcoat/cdn-manager
```


[1]: https://github.com/jetstack/cert-manager
[2]: https://aws.amazon.com/cloudfront/
