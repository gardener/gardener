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

//nolint:revive
package v1alpha1

import (
	"fmt"
	"unsafe"

	"github.com/gardener/gardener/pkg/apis/core"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("ControllerInstallation"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", core.RegistrationRefName, core.SeedRefName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", core.ShootSeedName, core.ShootCloudProfileName, core.ShootStatusSeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	// Add non-generated conversion functions
	if err := scheme.AddConversionFunc((*BackupBucket)(nil), (*core.BackupBucket)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_BackupBucket_To_core_BackupBucket(a.(*BackupBucket), b.(*core.BackupBucket), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*BackupBucketSpec)(nil), (*core.BackupBucketSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_BackupBucketSpec_To_core_BackupBucketSpec(a.(*BackupBucketSpec), b.(*core.BackupBucketSpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*BackupEntry)(nil), (*core.BackupEntry)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_BackupEntry_To_core_BackupEntry(a.(*BackupEntry), b.(*core.BackupEntry), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*BackupEntrySpec)(nil), (*core.BackupEntrySpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_BackupEntrySpec_To_core_BackupEntrySpec(a.(*BackupEntrySpec), b.(*core.BackupEntrySpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*Seed)(nil), (*core.Seed)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_Seed_To_core_Seed(a.(*Seed), b.(*core.Seed), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*SeedSpec)(nil), (*core.SeedSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_SeedSpec_To_core_SeedSpec(a.(*SeedSpec), b.(*core.SeedSpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*SeedNetworks)(nil), (*core.SeedNetworks)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_SeedNetworks_To_core_SeedNetworks(a.(*SeedNetworks), b.(*core.SeedNetworks), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*ShootStatus)(nil), (*core.ShootStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha1_ShootStatus_To_core_ShootStatus(a.(*ShootStatus), b.(*core.ShootStatus), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.BackupBucket)(nil), (*BackupBucket)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_BackupBucket_To_v1alpha1_BackupBucket(a.(*core.BackupBucket), b.(*BackupBucket), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.BackupBucketSpec)(nil), (*BackupBucketSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_BackupBucketSpec_To_v1alpha1_BackupBucketSpec(a.(*core.BackupBucketSpec), b.(*BackupBucketSpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.BackupEntry)(nil), (*BackupEntry)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_BackupEntry_To_v1alpha1_BackupEntry(a.(*core.BackupEntry), b.(*BackupEntry), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.BackupEntrySpec)(nil), (*BackupEntrySpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_BackupEntrySpec_To_v1alpha1_BackupEntrySpec(a.(*core.BackupEntrySpec), b.(*BackupEntrySpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.Seed)(nil), (*Seed)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_Seed_To_v1alpha1_Seed(a.(*core.Seed), b.(*Seed), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.SeedSpec)(nil), (*SeedSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_SeedSpec_To_v1alpha1_SeedSpec(a.(*core.SeedSpec), b.(*SeedSpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.SeedNetworks)(nil), (*SeedNetworks)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_SeedNetworks_To_v1alpha1_SeedNetworks(a.(*core.SeedNetworks), b.(*SeedNetworks), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*core.ShootStatus)(nil), (*ShootStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_core_ShootStatus_To_v1alpha1_ShootStatus(a.(*core.ShootStatus), b.(*ShootStatus), scope)
	}); err != nil {
		return err
	}
	return nil
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

func Convert_core_SeedStatus_To_v1alpha1_SeedStatus(in *core.SeedStatus, out *SeedStatus, s conversion.Scope) error {
	return autoConvert_core_SeedStatus_To_v1alpha1_SeedStatus(in, out, s)
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

func Convert_v1alpha1_ProjectSpec_To_core_ProjectSpec(in *ProjectSpec, out *core.ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_ProjectSpec_To_core_ProjectSpec(in, out, s); err != nil {
		return err
	}

	if owner := out.Owner; owner != nil {
	outer:
		for i, member := range out.Members {
			if member.Name == owner.Name && member.APIGroup == owner.APIGroup && member.Kind == owner.Kind {
				// add owner role to the current project's owner if not present
				for _, role := range member.Roles {
					if role == core.ProjectMemberOwner {
						continue outer
					}
				}

				out.Members[i].Roles = append(out.Members[i].Roles, core.ProjectMemberOwner)
			} else {
				// delete owner role from all other members
				out.Members[i].Roles = removeRoleFromRoles(member.Roles, ProjectMemberOwner)
			}
		}
	}

	return nil
}

func Convert_core_ProjectSpec_To_v1alpha1_ProjectSpec(in *core.ProjectSpec, out *ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_core_ProjectSpec_To_v1alpha1_ProjectSpec(in, out, s); err != nil {
		return err
	}

	if owner := out.Owner; owner != nil {
	outer:
		for i, member := range out.Members {
			if member.Name == owner.Name && member.APIGroup == owner.APIGroup && member.Kind == owner.Kind {
				// add owner role to the current project's owner if not present
				if member.Role == core.ProjectMemberOwner {
					// remove it from owners list if present
					out.Members[i].Roles = removeRoleFromRoles(member.Roles, ProjectMemberOwner)
					continue outer
				}
				for _, role := range member.Roles {
					if role == ProjectMemberOwner {
						continue outer
					}
				}

				if out.Members[i].Role == "" {
					out.Members[i].Role = core.ProjectMemberOwner
				} else {
					out.Members[i].Roles = append(out.Members[i].Roles, core.ProjectMemberOwner)
				}
			} else {
				// delete owner role from all other members
				out.Members[i].Roles = removeRoleFromRoles(member.Roles, ProjectMemberOwner)

				if member.Role == ProjectMemberOwner {
					if len(out.Members[i].Roles) == 0 {
						out.Members[i].Role = ""
					} else {
						out.Members[i].Role = out.Members[i].Roles[0]
						if len(out.Members[i].Roles) > 1 {
							out.Members[i].Roles = out.Members[i].Roles[1:]
						} else {
							out.Members[i].Roles = nil
						}
					}
				}
			}
		}
	}

	return nil
}

func Convert_v1alpha1_ProjectMember_To_core_ProjectMember(in *ProjectMember, out *core.ProjectMember, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_ProjectMember_To_core_ProjectMember(in, out, s); err != nil {
		return err
	}

	if len(in.Role) == 0 {
		return nil
	}

	// delete in.Role from out.Roles to make sure it gets added to the head
	if len(out.Roles) > 0 {
		out.Roles = removeRoleFromRoles(out.Roles, in.Role)
	}

	// add in.Role to the head of out.Roles
	out.Roles = append([]string{in.Role}, out.Roles...)

	return nil
}

func Convert_core_ProjectMember_To_v1alpha1_ProjectMember(in *core.ProjectMember, out *ProjectMember, s conversion.Scope) error {
	if err := autoConvert_core_ProjectMember_To_v1alpha1_ProjectMember(in, out, s); err != nil {
		return err
	}

	if len(in.Roles) > 0 {
		out.Role = in.Roles[0]
		out.Roles = in.Roles[1:]
	}

	return nil
}

func removeRoleFromRoles(roles []string, role string) []string {
	var newRoles []string
	for _, r := range roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	return newRoles
}
