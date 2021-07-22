// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func filterGardenSecret(obj interface{}) bool {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return false
	}
	if secret.Namespace != v1beta1constants.GardenNamespace {
		return false
	}
	return gardenRoleSelector.Matches(labels.Set(secret.Labels))
}

func newSecretEventHandler(ctx context.Context, gardenClient client.Client, logger logr.Logger) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(obj client.Object) []reconcile.Request {
		seedList := &gardencorev1beta1.SeedList{}
		if err := gardenClient.List(ctx, seedList); err != nil {
			logger.Error(err, "Could not enqueue seeds")
			return nil
		}

		requests := []reconcile.Request{}
		for _, seed := range seedList.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name: seed.Name,
				},
			})
		}

		return requests
	})
}
