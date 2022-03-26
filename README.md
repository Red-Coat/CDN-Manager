# CDN Manager

CDN Manager lets you manage your CDN Distributions via Kubernetes
resources, and syncs ingresses, load balancers and certificate settings
automatically for you.

It currently only support [AWS CloudFront][2], however support for other
platforms may be added at a later date.

Its technical design has drawn some inspiration from [cert-manager][1].


**This product is still in early alpha stage - not all features may be
supported, and things may change without notice.**


[1]: https://github.com/jetstack/cert-manager
[2]: https://aws.amazon.com/cloudfront/
