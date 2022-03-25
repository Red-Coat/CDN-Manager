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

package indexer

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	api "gitlab.com/redcoat/k8s-cdn-controller/pkg/api/v1alpha1"
)

// Returns the value of a field for a given Distribution to be used by
// the FieldIndexer
//
// This is a more specific func of the client.IndexerFunc, tailored to a
// kubernetes-compatible single key, and Distribution resources.
type IndexerFunc func(api.Distribution) string

// Indexes a new field on Distributions with the given index key and
// using the given IndexerFunc to extract the field
//
// In the event that the IndexerFunc returns an emtpty string, no fields
// will be indexed.
func NewIndexer(mgr ctrl.Manager, key string, indexer IndexerFunc) {
	ctx := context.TODO()
	mgr.GetFieldIndexer().IndexField(ctx, &api.Distribution{}, key,
		func(obj client.Object) []string {
			value := indexer(*obj.(*api.Distribution))
			if value != "" {
				return []string{value}
			} else {
				return []string{}
			}
		},
	)
}

// Returns the ObjectReference value of a field for a given Distribution
// to be used by the FieldIndexer
//
// This is a more specific func of the IndexerFunc above, tailored for
// fields with ObjectReference values.
type ObjReference func(api.Distribution) api.ObjectReference

// Indexes a new Object Reference field on Distributions
//
// If the ObjectReference is blank, or is not for the given type, an
// empty string is returned. Otherwise the Object Reference's name is
// returned.
func NewObjectReferenceIndexer(mgr ctrl.Manager, kind string, indexer ObjReference) {
	NewIndexer(mgr, kind, func(distro api.Distribution) string {
		if ref := indexer(distro); ref.Kind == kind {
			return ref.Name
		} else {
			return ""
		}
	})
}
