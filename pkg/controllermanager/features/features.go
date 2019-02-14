// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package features

import (
	"github.com/gardener/gardener/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

var (
	// FeatureGate is a shared global FeatureGate for Gardener Controller Manager flags.
	FeatureGate  = utilfeature.NewFeatureGate()
	featureGates = map[utilfeature.Feature]utilfeature.FeatureSpec{
		features.Logging:               {Default: false, PreRelease: utilfeature.Alpha},
		features.CertificateManagement: {Default: false, PreRelease: utilfeature.Alpha},
		features.VPA:                   {Default: false, PreRelease: utilfeature.Alpha},
	}
)

// RegisterFeatureGates registers the feature gates of the Gardener Controller Manager.
func RegisterFeatureGates() {
	FeatureGate.Add(featureGates)
}
