// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//nolint:revive
package v1beta1

import (
	"encoding/json"
	"fmt"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/core"
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
			case "metadata.name", "metadata.namespace", core.BackupEntrySeedName, core.BackupEntryBucketName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

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

	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("InternalSecret"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", core.InternalSecretType:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc(
		SchemeGroupVersion.WithKind("Project"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", core.ProjectNamespace:
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
			case "metadata.name", "metadata.namespace", core.ShootSeedName, core.ShootCloudProfileName, core.ShootCloudProfileRefName, core.ShootCloudProfileRefKind, core.ShootStatusSeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	return nil
}

func Convert_v1beta1_InternalSecret_To_core_InternalSecret(in *InternalSecret, out *core.InternalSecret, s conversion.Scope) error {
	if err := autoConvert_v1beta1_InternalSecret_To_core_InternalSecret(in, out, s); err != nil {
		return err
	}

	// StringData overwrites Data
	if len(in.StringData) > 0 {
		if out.Data == nil {
			out.Data = make(map[string][]byte, len(in.StringData))
		}

		for k, v := range in.StringData {
			out.Data[k] = []byte(v)
		}
	}

	return nil
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

func Convert_v1beta1_ControllerDeployment_To_core_ControllerDeployment(in *ControllerDeployment, out *core.ControllerDeployment, s conversion.Scope) error {
	if err := autoConvert_v1beta1_ControllerDeployment_To_core_ControllerDeployment(in, out, s); err != nil {
		return err
	}

	customType := false
	switch in.Type {
	case ControllerDeploymentTypeHelm:
		helmDeployment := &HelmControllerDeployment{}
		if len(in.ProviderConfig.Raw) > 0 {
			if err := json.Unmarshal(in.ProviderConfig.Raw, helmDeployment); err != nil {
				return err
			}
		}

		out.Helm = &core.HelmControllerDeployment{}
		if err := Convert_v1beta1_HelmControllerDeployment_To_core_HelmControllerDeployment(helmDeployment, out.Helm, s); err != nil {
			return err
		}
	default:
		customType = true
	}

	if !customType {
		// type and providerConfig are only used for custom types
		// built-in types are represented in the respective substructures
		out.Type = ""
		out.ProviderConfig = nil
	}

	return nil
}

func Convert_core_ControllerDeployment_To_v1beta1_ControllerDeployment(in *core.ControllerDeployment, out *ControllerDeployment, s conversion.Scope) error {
	if err := autoConvert_core_ControllerDeployment_To_v1beta1_ControllerDeployment(in, out, s); err != nil {
		return err
	}

	if in.Helm != nil {
		out.Type = ControllerDeploymentTypeHelm

		helmDeployment := &HelmControllerDeployment{}
		if err := Convert_core_HelmControllerDeployment_To_v1beta1_HelmControllerDeployment(in.Helm, helmDeployment, s); err != nil {
			return err
		}

		var err error
		out.ProviderConfig.Raw, err = json.Marshal(helmDeployment)
		if err != nil {
			return err
		}
	}

	return nil
}

func Convert_v1beta1_HelmControllerDeployment_To_core_HelmControllerDeployment(in *HelmControllerDeployment, out *core.HelmControllerDeployment, s conversion.Scope) error {
	if err := autoConvert_v1beta1_HelmControllerDeployment_To_core_HelmControllerDeployment(in, out, s); err != nil {
		return err
	}

	out.RawChart = in.Chart
	return nil
}

func Convert_core_HelmControllerDeployment_To_v1beta1_HelmControllerDeployment(in *core.HelmControllerDeployment, out *HelmControllerDeployment, s conversion.Scope) error {
	if err := autoConvert_core_HelmControllerDeployment_To_v1beta1_HelmControllerDeployment(in, out, s); err != nil {
		return err
	}

	out.Chart = in.RawChart
	return nil
}

func Convert_v1beta1_Capabilities_To_core_Capabilities(in *Capabilities, out *core.Capabilities, s conversion.Scope) error {
	for capabilityName, capabilityValues := range *in {
		coreCapabilityValues := core.CapabilityValues{
			Values: capabilityValues.Values,
		}
		(*out)[core.CapabilityName(capabilityName)] = coreCapabilityValues
	}
	return nil
}

func Convert_core_Capabilities_To_v1beta1_Capabilities(in *core.Capabilities, out *Capabilities, s conversion.Scope) error {
	for capabilityName, capabilityValues := range *in {
		v1betaCapabilityValues := CapabilityValues{
			Values: capabilityValues.Values,
		}
		(*out)[CapabilityName(capabilityName)] = v1betaCapabilityValues
	}

	return nil
}
