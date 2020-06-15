// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package v1beta1

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("BackupBucket"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", core.BackupBucketSeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("BackupEntry"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", core.BackupEntrySeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("Shoot"),
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
	return scheme.AddConversionFuncs()
}

func Convert_v1beta1_ProjectSpec_To_core_ProjectSpec(in *ProjectSpec, out *core.ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_v1beta1_ProjectSpec_To_core_ProjectSpec(in, out, s); err != nil {
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

func Convert_core_ProjectSpec_To_v1beta1_ProjectSpec(in *core.ProjectSpec, out *ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_core_ProjectSpec_To_v1beta1_ProjectSpec(in, out, s); err != nil {
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

func Convert_v1beta1_ProjectMember_To_core_ProjectMember(in *ProjectMember, out *core.ProjectMember, s conversion.Scope) error {
	if err := autoConvert_v1beta1_ProjectMember_To_core_ProjectMember(in, out, s); err != nil {
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

func Convert_core_ProjectMember_To_v1beta1_ProjectMember(in *core.ProjectMember, out *ProjectMember, s conversion.Scope) error {
	if err := autoConvert_core_ProjectMember_To_v1beta1_ProjectMember(in, out, s); err != nil {
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
