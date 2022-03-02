//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright (c) SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file

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

// Code generated by defaulter-gen. DO NOT EDIT.

package v1alpha1

import (
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// RegisterDefaults adds defaulters functions to the given scheme.
// Public to allow building arbitrary schemes.
// All generated defaulters are covering - they call all nested defaulters.
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&GardenletConfiguration{}, func(obj interface{}) { SetObjectDefaults_GardenletConfiguration(obj.(*GardenletConfiguration)) })
	return nil
}

func SetObjectDefaults_GardenletConfiguration(in *GardenletConfiguration) {
	SetDefaults_GardenletConfiguration(in)
	if in.GardenClientConnection != nil {
		SetDefaults_ClientConnectionConfiguration(&in.GardenClientConnection.ClientConnectionConfiguration)
	}
	if in.SeedClientConnection != nil {
		SetDefaults_ClientConnectionConfiguration(&in.SeedClientConnection.ClientConnectionConfiguration)
	}
	if in.ShootClientConnection != nil {
		SetDefaults_ClientConnectionConfiguration(&in.ShootClientConnection.ClientConnectionConfiguration)
	}
	if in.Controllers != nil {
		SetDefaults_GardenletControllerConfiguration(in.Controllers)
		if in.Controllers.BackupBucket != nil {
			SetDefaults_BackupBucketControllerConfiguration(in.Controllers.BackupBucket)
		}
		if in.Controllers.BackupEntry != nil {
			SetDefaults_BackupEntryControllerConfiguration(in.Controllers.BackupEntry)
		}
		if in.Controllers.BackupEntryMigration != nil {
			SetDefaults_BackupEntryMigrationControllerConfiguration(in.Controllers.BackupEntryMigration)
		}
		if in.Controllers.Bastion != nil {
			SetDefaults_BastionControllerConfiguration(in.Controllers.Bastion)
		}
		if in.Controllers.ControllerInstallation != nil {
			SetDefaults_ControllerInstallationControllerConfiguration(in.Controllers.ControllerInstallation)
		}
		if in.Controllers.ControllerInstallationCare != nil {
			SetDefaults_ControllerInstallationCareControllerConfiguration(in.Controllers.ControllerInstallationCare)
		}
		if in.Controllers.ControllerInstallationRequired != nil {
			SetDefaults_ControllerInstallationRequiredControllerConfiguration(in.Controllers.ControllerInstallationRequired)
		}
		if in.Controllers.Seed != nil {
			SetDefaults_SeedControllerConfiguration(in.Controllers.Seed)
		}
		if in.Controllers.Shoot != nil {
			SetDefaults_ShootControllerConfiguration(in.Controllers.Shoot)
		}
		if in.Controllers.ShootCare != nil {
			SetDefaults_ShootCareControllerConfiguration(in.Controllers.ShootCare)
			if in.Controllers.ShootCare.StaleExtensionHealthChecks != nil {
				SetDefaults_StaleExtensionHealthChecks(in.Controllers.ShootCare.StaleExtensionHealthChecks)
			}
		}
		if in.Controllers.ShootMigration != nil {
			SetDefaults_ShootMigrationControllerConfiguration(in.Controllers.ShootMigration)
		}
		if in.Controllers.ShootStateSync != nil {
			SetDefaults_ShootStateSyncControllerConfiguration(in.Controllers.ShootStateSync)
		}
		if in.Controllers.SeedAPIServerNetworkPolicy != nil {
			SetDefaults_SeedAPIServerNetworkPolicyControllerConfiguration(in.Controllers.SeedAPIServerNetworkPolicy)
		}
		if in.Controllers.ManagedSeed != nil {
			SetDefaults_ManagedSeedControllerConfiguration(in.Controllers.ManagedSeed)
		}
		if in.Controllers.ShootSecret != nil {
			SetDefaults_ShootSecretControllerConfiguration(in.Controllers.ShootSecret)
		}
	}
	if in.LeaderElection != nil {
		SetDefaults_LeaderElectionConfiguration(in.LeaderElection)
	}
	if in.SNI != nil {
		SetDefaults_SNI(in.SNI)
		if in.SNI.Ingress != nil {
			SetDefaults_SNIIngress(in.SNI.Ingress)
		}
	}
	for i := range in.ExposureClassHandlers {
		a := &in.ExposureClassHandlers[i]
		if a.SNI != nil {
			SetDefaults_SNI(a.SNI)
			if a.SNI.Ingress != nil {
				SetDefaults_SNIIngress(a.SNI.Ingress)
			}
		}
	}
}
