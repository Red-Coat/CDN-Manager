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
	corev1 "k8s.io/api/core/v1"

	api "gitlab.com/redcoat/k8s-cdn-controller/pkg/api/v1alpha1"
)

// Convienience function which returns a Distribution with default http
// and https ports set, and the host set from the given
// LoadBalancerIngress slice.
func DistributionFromIngress(
	class api.ObjectReference,
	ingress []corev1.LoadBalancerIngress,
) api.Distribution {
	return api.Distribution{
		Spec: api.DistributionSpec{
			DistributionClassRef: class,
			Origin: api.Origin{
				Host:      GetIngressHost(ingress),
				HTTPPort:  80,
				HTTPSPort: 443,
			},
		},
	}
}

// Checks to see if a LoadBalancerIngress[] resource has any values and
// uses this as the origin hostname if it does
func GetIngressHost(ingress []corev1.LoadBalancerIngress) string {
	if len(ingress) == 0 {
		return ""
	}

	// We currently only support one origin field
	firstHost := ingress[0]
	if firstHost.Hostname != "" {
		return firstHost.Hostname
	} else {
		return firstHost.IP
	}
}
