// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package rotation

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// ManagedByGardenerOperatorSecretsManager is the label selector for selecting secrets managed by gardener-operator's
// instance of secrets manager.
var ManagedByGardenerOperatorSecretsManager = client.MatchingLabels{
	"managed-by":       "secrets-manager",
	"manager-identity": "gardener-operator",
}

// ObservabilitySecretManagedByGardenerOperatorSecretsManager is the label selector for selecting observability secret managed by gardener-operator's
// instance of secrets manager.
var ObservabilitySecretManagedByGardenerOperatorSecretsManager = client.MatchingLabels{
	"managed-by":       "secrets-manager",
	"manager-identity": "gardener-operator",
	"name":             v1beta1constants.SecretNameObservabilityIngress,
}
