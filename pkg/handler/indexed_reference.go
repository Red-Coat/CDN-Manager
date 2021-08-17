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

package handler

import (
	"context"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	api "git.redcoat.dev/cdn/pkg/api/v1alpha1"
)

type EnqueueRequestForIndexedReference struct {
	client.Client

	ClusterScope bool

	Field string
}

// Create implements EventHandler.
func (e *EnqueueRequestForIndexedReference) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.Object, q)
}

// Update implements EventHandler.
func (e *EnqueueRequestForIndexedReference) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	switch {
	case evt.ObjectNew != nil:
		e.enqueue(evt.ObjectNew, q)
	case evt.ObjectOld != nil:
		e.enqueue(evt.ObjectOld, q)
	}
}

// Delete implements EventHandler.
func (e *EnqueueRequestForIndexedReference) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.Object, q)
}

// Generic implements EventHandler.
func (e *EnqueueRequestForIndexedReference) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.enqueue(evt.Object, q)
}

func (e *EnqueueRequestForIndexedReference) enqueue(
	object client.Object,
	q workqueue.RateLimitingInterface,
) {

	predicates := []client.ListOption{
		client.MatchingFields{e.Field: object.GetName()},
	}

	if !e.ClusterScope {
		predicates = append(predicates, client.InNamespace(object.GetNamespace()))
	}

	list := api.DistributionList{}
	e.List(context.TODO(), &list, predicates...)

	for _, item := range list.Items {
		q.Add(ctrl.Request{NamespacedName: client.ObjectKeyFromObject(&item)})
	}
}
