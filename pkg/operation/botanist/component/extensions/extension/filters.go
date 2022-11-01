// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extension

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

type extensionFilter func(e Extension) bool

func deployBeforeKubeAPIServer(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Reconcile == nil {
		return false
	}
	return *e.Lifecycle.Reconcile == gardencorev1beta1.BeforeKubeAPIServer ||
		*e.Lifecycle.Reconcile == gardencorev1beta1.BeforeAndAfterKubeAPIServer
}

func deployAfterKubeAPIServer(e Extension) bool {
	if e.Lifecycle == nil || e.Lifecycle.Reconcile == nil {
		return true
	}
	return *e.Lifecycle.Reconcile == gardencorev1beta1.AfterKubeAPIServer ||
		*e.Lifecycle.Reconcile == gardencorev1beta1.BeforeAndAfterKubeAPIServer
}
