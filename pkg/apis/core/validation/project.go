// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/api/validation/path"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/utils"
)

// ValidateProject validates a Project object.
func ValidateProject(project *core.Project) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&project.ObjectMeta, false, ValidateName, field.NewPath("metadata"))...)
	maxProjectNameLength := 10
	if len(project.Name) > maxProjectNameLength {
		allErrs = append(allErrs, field.TooLong(field.NewPath("metadata", "name"), project.Name, maxProjectNameLength))
	}
	allErrs = append(allErrs, validateNameConsecutiveHyphens(project.Name, field.NewPath("metadata", "name"))...)
	allErrs = append(allErrs, ValidateProjectSpec(&project.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateProjectUpdate validates a Project object before an update.
func ValidateProjectUpdate(newProject, oldProject *core.Project) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newProject.ObjectMeta, &oldProject.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateProject(newProject)...)

	if oldProject.Spec.CreatedBy != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newProject.Spec.CreatedBy, oldProject.Spec.CreatedBy, field.NewPath("spec", "createdBy"))...)
	}
	if oldProject.Spec.Namespace != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newProject.Spec.Namespace, oldProject.Spec.Namespace, field.NewPath("spec", "namespace"))...)
	}
	if oldProject.Spec.Owner != nil && newProject.Spec.Owner == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "owner"), newProject.Spec.Owner, "owner cannot be reset"))
	}

	return allErrs
}

// ValidateProjectSpec validates the specification of a Project object.
func ValidateProjectSpec(projectSpec *core.ProjectSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	reservedNamespaceNames := []string{core.GardenerSeedLeaseNamespace, core.GardenerShootIssuerNamespace, core.GardenerSystemPublicNamespace}
	if projectSpec.Namespace != nil && slices.Contains(reservedNamespaceNames, *projectSpec.Namespace) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("namespace"), projectSpec.Namespace, fmt.Sprintf("Project namespaces %q are reserved by Gardener", reservedNamespaceNames)))
	}
	ownerFound := false

	members := make(map[string]struct{}, len(projectSpec.Members))

	for i, member := range projectSpec.Members {
		idxPath := fldPath.Child("members").Index(i)

		apiGroup, kind, namespace, name, err := ProjectMemberProperties(member)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("name"), member.Name, err.Error()))
			continue
		}
		id := ProjectMemberId(apiGroup, kind, namespace, name)

		if _, ok := members[id]; ok {
			allErrs = append(allErrs, field.Duplicate(idxPath, member))
		} else {
			members[id] = struct{}{}
		}

		allErrs = append(allErrs, ValidateProjectMember(member, idxPath)...)

		for j, role := range member.Roles {
			if role == core.ProjectMemberOwner {
				if ownerFound {
					allErrs = append(allErrs, field.Forbidden(idxPath.Child("roles").Index(j), "cannot have more than one member having the owner role"))
				} else {
					ownerFound = true
				}
			}
		}
	}
	if createdBy := projectSpec.CreatedBy; createdBy != nil {
		allErrs = append(allErrs, ValidateSubject(*createdBy, fldPath.Child("createdBy"))...)
	}
	if owner := projectSpec.Owner; owner != nil {
		allErrs = append(allErrs, ValidateSubject(*owner, fldPath.Child("owner"))...)
	}
	if description := projectSpec.Description; description != nil && len(*description) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("description"), "must provide a description when key is present"))
	}
	if purpose := projectSpec.Purpose; purpose != nil && len(*purpose) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("purpose"), "must provide a purpose when key is present"))
	}

	if projectSpec.Tolerations != nil {
		allErrs = append(allErrs, ValidateTolerations(projectSpec.Tolerations.Defaults, fldPath.Child("tolerations", "defaults"))...)
		allErrs = append(allErrs, ValidateTolerations(projectSpec.Tolerations.Whitelist, fldPath.Child("tolerations", "whitelist"))...)
		allErrs = append(allErrs, ValidateTolerationsAgainstAllowlist(projectSpec.Tolerations.Defaults, projectSpec.Tolerations.Whitelist, fldPath.Child("tolerations", "defaults"))...)
	}

	allErrs = append(allErrs, validateDualApprovalForDeletion(projectSpec.DualApprovalForDeletion, fldPath.Child("dualApprovalForDeletion"))...)

	return allErrs
}

// ValidateSubject validates the subject representing the owner.
func ValidateSubject(subject rbacv1.Subject, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(subject.Name) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), ""))
	}

	switch subject.Kind {
	case rbacv1.ServiceAccountKind:
		if len(subject.Name) > 0 {
			for _, msg := range apivalidation.ValidateServiceAccountName(subject.Name, false) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), subject.Name, msg))
			}
		}
		if len(subject.APIGroup) > 0 {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiGroup"), subject.APIGroup, []string{""}))
		}
		if len(subject.Namespace) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("namespace"), ""))
		}

	case rbacv1.UserKind, rbacv1.GroupKind:
		if subject.APIGroup != rbacv1.GroupName {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiGroup"), subject.APIGroup, []string{rbacv1.GroupName}))
		}

	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), subject.Kind, []string{rbacv1.ServiceAccountKind, rbacv1.UserKind, rbacv1.GroupKind}))
	}

	return allErrs
}

var supportedRoles = sets.New(
	core.ProjectMemberOwner,
	core.ProjectMemberAdmin,
	core.ProjectMemberViewer,
	core.ProjectMemberUserAccessManager,
	core.ProjectMemberServiceAccountManager,
)

const extensionRoleMaxLength = 20

// ValidateProjectMember validates the specification of a Project member.
func ValidateProjectMember(member core.ProjectMember, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateSubject(member.Subject, fldPath)...)

	foundRoles := make(map[string]struct{}, len(member.Roles))
	for i, role := range member.Roles {
		rolesPath := fldPath.Child("roles").Index(i)

		if _, ok := foundRoles[role]; ok {
			allErrs = append(allErrs, field.Duplicate(rolesPath, role))
		}
		foundRoles[role] = struct{}{}

		if !supportedRoles.Has(role) && !strings.HasPrefix(role, core.ProjectMemberExtensionPrefix) {
			allErrs = append(allErrs, field.NotSupported(rolesPath, role, append(sets.List(supportedRoles), core.ProjectMemberExtensionPrefix+"*")))
		}

		if strings.HasPrefix(role, core.ProjectMemberExtensionPrefix) {
			extensionRoleName := strings.TrimPrefix(role, core.ProjectMemberExtensionPrefix)

			if len(extensionRoleName) > extensionRoleMaxLength {
				allErrs = append(allErrs, field.TooLong(rolesPath, role, extensionRoleMaxLength))
			}

			// the extension role name will be used as part of a ClusterRole name
			if errs := path.IsValidPathSegmentName(extensionRoleName); len(errs) > 0 {
				allErrs = append(allErrs, field.Invalid(rolesPath, role, strings.Join(errs, ", ")))
			}
		}
	}

	return allErrs
}

// ValidateTolerations validates the given tolerations.
func ValidateTolerations(tolerations []core.Toleration, fldPath *field.Path) field.ErrorList {
	var (
		allErrs   field.ErrorList
		keyValues = sets.New[string]()
	)

	for i, toleration := range tolerations {
		idxPath := fldPath.Index(i)

		if len(toleration.Key) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("key"), "cannot be empty"))
		}

		id := utils.IDForKeyWithOptionalValue(toleration.Key, toleration.Value)
		if keyValues.Has(id) || keyValues.Has(toleration.Key) {
			allErrs = append(allErrs, field.Duplicate(idxPath, id))
		}
		keyValues.Insert(id)
	}

	return allErrs
}

// ValidateTolerationsAgainstAllowlist validates the given tolerations against the given allowlist.
func ValidateTolerationsAgainstAllowlist(tolerations, allowlist []core.Toleration, fldPath *field.Path) field.ErrorList {
	var (
		allErrs            field.ErrorList
		allowedTolerations = sets.New[string]()
	)

	for _, toleration := range allowlist {
		allowedTolerations.Insert(utils.IDForKeyWithOptionalValue(toleration.Key, toleration.Value))
	}

	for i, toleration := range tolerations {
		id := utils.IDForKeyWithOptionalValue(toleration.Key, toleration.Value)
		if !allowedTolerations.Has(utils.IDForKeyWithOptionalValue(toleration.Key, nil)) && !allowedTolerations.Has(id) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Index(i), fmt.Sprintf("only the following tolerations are allowed: %+v", allowedTolerations.UnsortedList())))
		}
	}

	return allErrs
}

func validateDualApprovalForDeletion(dualApproval []core.DualApprovalForDeletion, fldPath *field.Path) field.ErrorList {
	var (
		allErrs            field.ErrorList
		resources          = sets.New[string]()
		supportedResources = []string{"shoots"}
	)

	for i, cfg := range dualApproval {
		idxPath := fldPath.Index(i)

		if len(cfg.Resource) == 0 {
			allErrs = append(allErrs, field.Required(idxPath.Child("resource"), "cannot be empty"))
		} else {
			if !slices.Contains(supportedResources, cfg.Resource) {
				allErrs = append(allErrs, field.NotSupported(idxPath.Child("resource"), cfg.Resource, supportedResources))
			}

			if resources.Has(cfg.Resource) {
				allErrs = append(allErrs, field.Duplicate(idxPath.Child("resource"), cfg.Resource))
			}
			resources.Insert(cfg.Resource)
		}

		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&cfg.Selector, metav1validation.LabelSelectorValidationOptions{}, idxPath.Child("selector"))...)
	}

	return allErrs
}

// ValidateProjectStatusUpdate validates the status field of a Project object.
func ValidateProjectStatusUpdate(newProject, oldProject *core.Project) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(oldProject.Status.Phase) > 0 && len(newProject.Status.Phase) == 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status").Child("phase"), newProject.Status.Phase, "phase cannot be updated to an empty string"))
	}

	return allErrs
}

// ProjectMemberProperties returns the properties for the given project member.
func ProjectMemberProperties(member core.ProjectMember) (string, string, string, string, error) {
	var (
		apiGroup  = member.APIGroup
		kind      = member.Kind
		namespace = member.Namespace
		name      = member.Name
	)

	if member.Kind == rbacv1.UserKind && strings.HasPrefix(member.Name, serviceaccount.ServiceAccountUsernamePrefix) {
		user := strings.Split(member.Name, serviceaccount.ServiceAccountUsernameSeparator)
		if len(user) < 4 {
			return "", "", "", "", fmt.Errorf("unsupported service account user name: %q", member.Name)
		}

		apiGroup = ""
		kind = rbacv1.ServiceAccountKind
		namespace = user[2]
		name = user[3]
	}

	return apiGroup, kind, namespace, name, nil
}

// ProjectMemberId returns an internal ID for the project member.
func ProjectMemberId(apiGroup, kind, namespace, name string) string {
	return fmt.Sprintf("%s_%s_%s_%s", apiGroup, kind, namespace, name)
}
