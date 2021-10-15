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

package predicate

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// HasOperationAnnotation is a predicate for the operation annotation.
func HasOperationAnnotation() predicate.Predicate {
	return FromMapper(MapperFunc(func(e event.GenericEvent) bool {
		return e.Object.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationReconcile ||
			e.Object.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationRestore ||
			e.Object.GetAnnotations()[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate
	}), CreateTrigger, UpdateNewTrigger, GenericTrigger)
}
