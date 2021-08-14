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

package util

import (
	"regexp"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Converts a string in the form namespace/name into a client.ObjectKey
//
// Will return nil if given an empty string.
func ObjectKeyFromString(key string) *client.ObjectKey {
	if key == "" {
		return nil
	}

	re := regexp.MustCompile("/")
	parts := re.Split(key, 2)

	return &client.ObjectKey{
		Namespace: parts[0],
		Name:      parts[1],
	}
}
