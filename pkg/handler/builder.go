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
	"reflect"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

func BuildIndexedReferenceWatcher(
	client client.Client,
	obj client.Object,
) (source.Source, handler.EventHandler) {
	name := reflect.TypeOf(obj).Elem().Name()

	return &source.Kind{Type: obj}, &EnqueueRequestForIndexedReference{
		Client:       client,
		Field:        name,
		ClusterScope: strings.HasPrefix(name, "Cluster"),
	}
}
