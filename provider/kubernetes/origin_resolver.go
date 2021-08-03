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

package kubernetes

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	api "redcoat.dev/cdn/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type OriginResolver struct {
	client.Client
	Origin    api.Origin
	Resolved  *ResolvedOrigin
	Namespace string
}

type ResolvedOrigin struct {
	Host      string
	HTTPPort  int32
	HTTPSPort int32
}

func (r *OriginResolver) ResolveCustomHost() {
	if r.Origin.Host != "" {
		r.Resolved.Host = r.Origin.Host
	}
}

func ResolveCustomPort(port *api.ServicePort, dest *int32) {
	if port != nil && port.Number != 0 {
		*dest = port.Number
	}
}

func (r *OriginResolver) IsComplete() bool {
	return r.Resolved.Host != "" && r.Resolved.HTTPPort != 0 && r.Resolved.HTTPSPort != 0
}

func SetPort(port corev1.ServicePort, portSpec *api.ServicePort, dest *int32) {
	if portSpec != nil && port.Name == portSpec.Name {
		*dest = port.Port
	}
}

func (r *OriginResolver) LoadResource(obj client.Object) {
	r.Get(context.TODO(), client.ObjectKey{
		Namespace: r.Namespace,
		Name:      r.Origin.Target.Name,
	}, obj)
}

func (r *OriginResolver) ResolveLoadBalancer(ingress []corev1.LoadBalancerIngress) error {
	// If the Host is already set, it must have been via the custom field,
	// which is the first thing that is checked. This field takes
	// precedence over autodiscovered ones so we can skip this check here.
	if r.Resolved.Host != "" {
		return nil
	}

	if len(ingress) == 0 {
		return fmt.Errorf("There is no load balancer ingress on the service")
	}

	// We currently only support one origin field
	firstHost := ingress[0]
	if firstHost.Hostname != "" {
		r.Resolved.Host = firstHost.Hostname
	} else {
		r.Resolved.Host = firstHost.IP
	}

	return nil
}

func (r *OriginResolver) ResolveService() error {
	var svc corev1.Service
	r.LoadResource(&svc)

	if err := r.ResolveLoadBalancer(svc.Status.LoadBalancer.Ingress); err != nil {
		return err
	}

	for _, port := range svc.Spec.Ports {
		SetPort(port, r.Origin.HTTPPort, &r.Resolved.HTTPPort)
		SetPort(port, r.Origin.HTTPSPort, &r.Resolved.HTTPSPort)
	}

	return nil
}

func (r *OriginResolver) ResolveIngress() error {
	var ing networking.Ingress
	r.LoadResource(&ing)

	return r.ResolveLoadBalancer(ing.Status.LoadBalancer.Ingress)
}

func (r *OriginResolver) Resolve(distro api.Distribution) (*ResolvedOrigin, error) {
	r.Origin = distro.Spec.Origin
	r.Resolved = &ResolvedOrigin{}

	r.ResolveCustomHost()
	ResolveCustomPort(r.Origin.HTTPPort, &r.Resolved.HTTPPort)
	ResolveCustomPort(r.Origin.HTTPSPort, &r.Resolved.HTTPSPort)

	if r.IsComplete() {
		return r.Resolved, nil
	}

	if r.Origin.Target != nil {
		var err error
		if r.Origin.Target.Kind == "Service" {
			err = r.ResolveService()
		} else {
			err = r.ResolveIngress()
		}

		if err != nil {
			return nil, err
		}
	}

	if !r.IsComplete() {
		return r.Resolved, fmt.Errorf("Not all information was provided")
	}

	return r.Resolved, nil
}
