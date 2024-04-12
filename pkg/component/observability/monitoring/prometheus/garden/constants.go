// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package garden

import (
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	// Label is a constant for the label of the garden prometheus instance.
	Label = "garden"
	// ServiceAccountName is the name of the service account in the virtual garden cluster.
	ServiceAccountName = "prometheus-" + Label
	// AccessSecretName is the name of the secret containing a token for accessing the virtual garden cluster.
	AccessSecretName = gardenerutils.SecretNamePrefixShootAccess + ServiceAccountName
)
