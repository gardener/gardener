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

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

// FeatureGate is a shared global FeatureGate for Gardenlet flags.
var FeatureGate = featuregate.NewFeatureGate()

// RegisterFeatureGates registers the feature gates of the Gardenlet.
func RegisterFeatureGates() {
	utilruntime.Must(FeatureGate.Add(features.GetFeatures(
		features.HVPA,
		features.HVPAForShootedSeed,
		features.ManagedIstio,
		features.APIServerSNI,
		features.SeedKubeScheduler,
		features.ReversedVPN,
		features.CopyEtcdBackupsDuringControlPlaneMigration,
		features.ForceRestore,
		features.DisableDNSProviderManagement,
		features.ShootCARotation,
		features.ShootSARotation,
		features.HAControlPlanes,
		features.DefaultSeccompProfile,
	)))
}
