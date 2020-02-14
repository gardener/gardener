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
				for j, role := range member.Roles {
					if role == ProjectMemberOwner {
						out.Members[i].Roles = append(out.Members[i].Roles[:j], out.Members[i].Roles[j+1:]...)
					}
				}
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
				for _, role := range member.Roles {
					if role == ProjectMemberOwner {
						continue outer
					}
				}

				out.Members[i].Roles = append(out.Members[i].Roles, ProjectMemberOwner)
			} else {
				// delete owner role from all other members
				for j, role := range member.Roles {
					if role == ProjectMemberOwner {
						out.Members[i].Roles = append(out.Members[i].Roles[:j], out.Members[i].Roles[j+1:]...)
					}
				}

				if member.Role != nil && *member.Role == ProjectMemberOwner {
					if len(out.Members[i].Roles) > 0 {
						out.Members[i].Role = &out.Members[i].Roles[0]
					} else {
						out.Members[i].Role = nil
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

	if in.Role == nil {
		return nil
	}

	for _, role := range in.Roles {
		if role == *in.Role {
			return nil
		}
	}

	out.Roles = append([]string{*in.Role}, in.Roles...)

	return nil
}

func Convert_core_ProjectMember_To_v1beta1_ProjectMember(in *core.ProjectMember, out *ProjectMember, s conversion.Scope) error {
	if err := autoConvert_core_ProjectMember_To_v1beta1_ProjectMember(in, out, s); err != nil {
		return err
	}

	if len(in.Roles) > 0 {
		out.Role = &in.Roles[0]
	}

	return nil
}
