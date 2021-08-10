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

package resolver

import (
	"context"
	"fmt"
	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OriginResolver struct {
	client.Client
	Origin    api.Origin
	Resolved  *ResolvedOrigin
	Namespace string
}

// Represents the system's "resolved" origin details
//
// Distributions can point to Ingresses, Services, or custom hosts, on
// any ports you specify. When it comes to the providers performing
// their tasks, they need to know the actual hostname / port
// configurations to give to their CDN APIs.
type ResolvedOrigin struct {
	Host      string
	HTTPPort  int32
	HTTPSPort int32
}

// Checks to see if a custom hostname has been specified - if it has,
// this takes precedence and is immediately set
func (r *OriginResolver) resolveCustomHost() {
	if r.Origin.Host != "" {
		r.Resolved.Host = r.Origin.Host
	}
}

// Checks to see if a port number has been given for the given port - if
// it has, this takes precedence and is immediately set
func resolveCustomPort(port *api.ServicePort, dest *int32) {
	if port != nil && port.Number != 0 {
		*dest = port.Number
	}
}

// Checks to see if the given named Service Port matches a the name of a
// port on the Distribution - if it does we'll use its value
func setPort(port corev1.ServicePort, portSpec *api.ServicePort, dest *int32) {
	if portSpec != nil && port.Name == portSpec.Name {
		*dest = port.Port
	}
}

// Loads a resource (either Ingress or Service) by looking for the
// resource by the name given in the Distribution resource, and in the
// same namespace
func (r *OriginResolver) loadResource(obj client.Object) {
	r.Get(context.TODO(), client.ObjectKey{
		Namespace: r.Namespace,
		Name:      r.Origin.Target.Name,
	}, obj)
}

// Checks to see if a LoadBalancerIngress[] resource has any values and
// uses this as the origin hostname if it does
func (r *OriginResolver) resolveLoadBalancer(ingress []corev1.LoadBalancerIngress) {
	// If the Host is already set, it must have been via the custom field,
	// which is the first thing that is checked. This field takes
	// precedence over autodiscovered ones so we can skip this check here.
	if r.Resolved.Host != "" || len(ingress) == 0 {
		return
	}

	// We currently only support one origin field
	firstHost := ingress[0]
	if firstHost.Hostname != "" {
		r.Resolved.Host = firstHost.Hostname
	} else {
		r.Resolved.Host = firstHost.IP
	}
}

// Loads a Service Resource and tries to infer origin details from it
//
// If the Service has a status.loadBalancer.ingress[0].hostname/ip, this
// will be used as the origin.
// The service's named ports will be checked against the named ports on
// the Distribution
func (r *OriginResolver) resolveService() {
	var svc corev1.Service
	r.loadResource(&svc)

	r.resolveLoadBalancer(svc.Status.LoadBalancer.Ingress)

	for _, port := range svc.Spec.Ports {
		setPort(port, r.Origin.HTTPPort, &r.Resolved.HTTPPort)
		setPort(port, r.Origin.HTTPSPort, &r.Resolved.HTTPSPort)
	}
}

// Loads an Ingress Resoruce and tries to infer origin hostname from its
// loadbalancer, if it is set
func (r *OriginResolver) resolveIngress() {
	var ing networking.Ingress
	r.loadResource(&ing)

	r.resolveLoadBalancer(ing.Status.LoadBalancer.Ingress)
}

// Inspects a Distribution and tries to resolve its origin details from
// it
func (r *OriginResolver) Resolve(distro api.Distribution) (ResolvedOrigin, error) {
	r.Origin = distro.Spec.Origin
	r.Namespace = distro.Namespace
	r.Resolved = &ResolvedOrigin{}

	r.resolveCustomHost()
	resolveCustomPort(r.Origin.HTTPPort, &r.Resolved.HTTPPort)
	resolveCustomPort(r.Origin.HTTPSPort, &r.Resolved.HTTPSPort)

	if r.Resolved.Host != "" && r.Resolved.HTTPPort != 0 && r.Resolved.HTTPSPort != 0 {
		return *r.Resolved, nil
	}

	if r.Origin.Target != nil {
		if r.Origin.Target.Kind == "Service" {
			r.resolveService()
		} else {
			r.resolveIngress()
		}
	}

	if r.Resolved.HTTPPort == 0 {
		r.Resolved.HTTPPort = 80
	}
	if r.Resolved.HTTPSPort == 0 {
		r.Resolved.HTTPSPort = 443
	}

	if r.Resolved.Host == "" {
		return *r.Resolved, fmt.Errorf("Could not determine origin hostname, please provide this")
	}

	return *r.Resolved, nil
}
