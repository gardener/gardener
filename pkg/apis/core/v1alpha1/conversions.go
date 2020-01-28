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

package v1alpha1

import (
	"fmt"
	"unsafe"

	"github.com/gardener/gardener/pkg/apis/core"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", core.ShootSeedName, core.ShootCloudProfileName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	// Add non-generated conversion functions
	return scheme.AddConversionFuncs(
		Convert_v1alpha1_BackupBucket_To_core_BackupBucket,
		Convert_v1alpha1_BackupBucketSpec_To_core_BackupBucketSpec,
		Convert_v1alpha1_BackupEntry_To_core_BackupEntry,
		Convert_v1alpha1_BackupEntrySpec_To_core_BackupEntrySpec,
		Convert_v1alpha1_Seed_To_core_Seed,
		Convert_v1alpha1_SeedSpec_To_core_SeedSpec,
		Convert_v1alpha1_SeedNetworks_To_core_SeedNetworks,
		Convert_v1alpha1_ShootStatus_To_core_ShootStatus,
		Convert_core_BackupBucket_To_v1alpha1_BackupBucket,
		Convert_core_BackupBucketSpec_To_v1alpha1_BackupBucketSpec,
		Convert_core_BackupEntry_To_v1alpha1_BackupEntry,
		Convert_core_BackupEntrySpec_To_v1alpha1_BackupEntrySpec,
		Convert_core_Seed_To_v1alpha1_Seed,
		Convert_core_SeedSpec_To_v1alpha1_SeedSpec,
		Convert_core_SeedNetworks_To_v1alpha1_SeedNetworks,
		Convert_core_ShootStatus_To_v1alpha1_ShootStatus,
	)
}

func Convert_v1alpha1_BackupBucket_To_core_BackupBucket(in *BackupBucket, out *core.BackupBucket, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_BackupBucket_To_core_BackupBucket(in, out, s); err != nil {
		return err
	}

	out.Spec.SeedName = in.Spec.Seed

	return nil
}

func Convert_core_BackupBucket_To_v1alpha1_BackupBucket(in *core.BackupBucket, out *BackupBucket, s conversion.Scope) error {
	if err := autoConvert_core_BackupBucket_To_v1alpha1_BackupBucket(in, out, s); err != nil {
		return err
	}

	out.Spec.Seed = in.Spec.SeedName

	return nil
}

func Convert_core_BackupBucketSpec_To_v1alpha1_BackupBucketSpec(in *core.BackupBucketSpec, out *BackupBucketSpec, s conversion.Scope) error {
	return autoConvert_core_BackupBucketSpec_To_v1alpha1_BackupBucketSpec(in, out, s)
}

func Convert_v1alpha1_BackupBucketSpec_To_core_BackupBucketSpec(in *BackupBucketSpec, out *core.BackupBucketSpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_BackupBucketSpec_To_core_BackupBucketSpec(in, out, s)
}

func Convert_v1alpha1_BackupEntry_To_core_BackupEntry(in *BackupEntry, out *core.BackupEntry, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_BackupEntry_To_core_BackupEntry(in, out, s); err != nil {
		return err
	}

	out.Spec.SeedName = in.Spec.Seed

	return nil
}

func Convert_core_BackupEntry_To_v1alpha1_BackupEntry(in *core.BackupEntry, out *BackupEntry, s conversion.Scope) error {
	if err := autoConvert_core_BackupEntry_To_v1alpha1_BackupEntry(in, out, s); err != nil {
		return err
	}

	out.Spec.Seed = in.Spec.SeedName

	return nil
}

func Convert_core_BackupEntrySpec_To_v1alpha1_BackupEntrySpec(in *core.BackupEntrySpec, out *BackupEntrySpec, s conversion.Scope) error {
	return autoConvert_core_BackupEntrySpec_To_v1alpha1_BackupEntrySpec(in, out, s)
}

func Convert_v1alpha1_BackupEntrySpec_To_core_BackupEntrySpec(in *BackupEntrySpec, out *core.BackupEntrySpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_BackupEntrySpec_To_core_BackupEntrySpec(in, out, s)
}

func Convert_v1alpha1_Seed_To_core_Seed(in *Seed, out *core.Seed, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_Seed_To_core_Seed(in, out, s); err != nil {
		return err
	}

	out.Spec.Networks.BlockCIDRs = in.Spec.BlockCIDRs

	return nil
}

func Convert_core_Seed_To_v1alpha1_Seed(in *core.Seed, out *Seed, s conversion.Scope) error {
	if err := autoConvert_core_Seed_To_v1alpha1_Seed(in, out, s); err != nil {
		return err
	}

	out.Spec.BlockCIDRs = in.Spec.Networks.BlockCIDRs

	return nil
}

func Convert_core_SeedSpec_To_v1alpha1_SeedSpec(in *core.SeedSpec, out *SeedSpec, s conversion.Scope) error {
	return autoConvert_core_SeedSpec_To_v1alpha1_SeedSpec(in, out, s)
}

func Convert_v1alpha1_SeedSpec_To_core_SeedSpec(in *SeedSpec, out *core.SeedSpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_SeedSpec_To_core_SeedSpec(in, out, s)
}

func Convert_core_SeedNetworks_To_v1alpha1_SeedNetworks(in *core.SeedNetworks, out *SeedNetworks, s conversion.Scope) error {
	return autoConvert_core_SeedNetworks_To_v1alpha1_SeedNetworks(in, out, s)
}

func Convert_v1alpha1_SeedNetworks_To_core_SeedNetworks(in *SeedNetworks, out *core.SeedNetworks, s conversion.Scope) error {
	return autoConvert_v1alpha1_SeedNetworks_To_core_SeedNetworks(in, out, s)
}

func Convert_core_ShootStatus_To_v1alpha1_ShootStatus(in *core.ShootStatus, out *ShootStatus, s conversion.Scope) error {
	if err := autoConvert_core_ShootStatus_To_v1alpha1_ShootStatus(in, out, s); err != nil {
		return err
	}

	if len(in.LastErrors) != 0 {
		out.LastError = (*LastError)(unsafe.Pointer(&in.LastErrors[0]))
		if len(in.LastErrors) > 1 {
			lastErrors := in.LastErrors[1:]
			out.LastErrors = *(*[]LastError)(unsafe.Pointer(&lastErrors))
		} else {
			out.LastErrors = nil
		}
	}

	out.Seed = in.SeedName

	return nil
}

func Convert_v1alpha1_ShootStatus_To_core_ShootStatus(in *ShootStatus, out *core.ShootStatus, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_ShootStatus_To_core_ShootStatus(in, out, s); err != nil {
		return err
	}

	if in.LastError != nil {
		outLastErrors := []core.LastError{
			{
				Description:    in.LastError.Description,
				Codes:          *(*[]core.ErrorCode)(unsafe.Pointer(&in.LastError.Codes)),
				LastUpdateTime: in.LastError.LastUpdateTime,
			},
		}
		out.LastErrors = append(outLastErrors, *(*[]core.LastError)(unsafe.Pointer(&in.LastErrors))...)
	} else {
		out.LastErrors = nil
	}

	out.SeedName = in.Seed

	return nil
}
