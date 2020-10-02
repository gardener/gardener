// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ObjectName returns the name of the given object in the format <namespace>/<name>
func ObjectName(obj runtime.Object) string {
	k, err := client.ObjectKeyFromObject(obj)
	if err != nil {
		return "/"
	}
	return k.String()
}

// DeleteObjects deletes a list of Kubernetes objects.
func DeleteObjects(ctx context.Context, c client.Client, objects ...runtime.Object) error {
	for _, obj := range objects {
		if err := DeleteObject(ctx, c, obj); err != nil {
			return err
		}
	}
	return nil
}

// DeleteObject deletes a Kubernetes object. It ignores 'not found' and 'no match' errors.
func DeleteObject(ctx context.Context, c client.Client, object runtime.Object) error {
	if err := c.Delete(ctx, object); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
		return err
	}
	return nil
}
