// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"net"
	"time"

	"golang.org/x/crypto/ssh"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/operations"
)

// ValidateBastion validates a Bastion object.
func ValidateBastion(bastion *operations.Bastion) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&bastion.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateBastionSpec(&bastion.Spec, field.NewPath("spec"))...)

	return allErrs
}

// ValidateBastionUpdate validates a Bastion object before an update.
func ValidateBastionUpdate(newBastion, oldBastion *operations.Bastion) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newBastion.ObjectMeta, &oldBastion.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newBastion.Annotations[v1beta1constants.GardenCreatedBy], oldBastion.Annotations[v1beta1constants.GardenCreatedBy], field.NewPath("metadata.annotations"))...)

	allErrs = append(allErrs, ValidateBastionSpecUpdate(&newBastion.Spec, &oldBastion.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateBastion(newBastion)...)

	return allErrs
}

// ValidateBastionSpec validates the specification of a Bastion object.
func ValidateBastionSpec(spec *operations.BastionSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(spec.ShootRef.Name) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("shootRef.name"), spec.ShootRef.Name, "shoot reference must not be empty"))
	}

	if len(spec.SSHPublicKey) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("sshPublicKey"), spec.SSHPublicKey, "sshPublicKey must not be empty"))
	} else if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(spec.SSHPublicKey)); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("sshPublicKey"), spec.SSHPublicKey, "invalid sshPublicKey"))
	}

	if len(spec.Ingress) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ingress"), spec.Ingress, "ingress must not be empty"))
	}

	for _, block := range spec.Ingress {
		if len(block.IPBlock.CIDR) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ingress"), block.IPBlock.CIDR, "CIDR must not be empty"))
		} else if _, _, err := net.ParseCIDR(block.IPBlock.CIDR); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ingress"), block.IPBlock.CIDR, "invalid CIDR"))
		}
	}

	if spec.Machine != nil {
		if spec.Machine.Type == nil && spec.Machine.Image == nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("machine"), spec.Machine, "at least one of type or image must be specified"))
		}
		if spec.Machine.Type != nil && len(*spec.Machine.Type) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("machine", "type"), *spec.Machine.Type, "machine type must not be empty"))
		}
		if spec.Machine.Image != nil {
			if len(spec.Machine.Image.Name) == 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("machine", "image", "name"), spec.Machine.Image.Name, "machine image name must not be empty"))
			}
			if spec.Machine.Image.Version != nil && len(*spec.Machine.Image.Version) == 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("machine", "image", "version"), *spec.Machine.Image.Version, "machine image version must not be empty"))
			}
		}
	}

	return allErrs
}

// ValidateBastionSpecUpdate validates the specification of a Bastion object.
func ValidateBastionSpecUpdate(newSpec, oldSpec *operations.BastionSpec, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.ShootRef.Name, oldSpec.ShootRef.Name, fldPath.Child("shootRef.name"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.SSHPublicKey, oldSpec.SSHPublicKey, fldPath.Child("sshPublicKey"))...)
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Machine, oldSpec.Machine, fldPath.Child("machine"))...)

	return allErrs
}

// ValidateBastionStatusUpdate validates the status field of a Bastion object.
func ValidateBastionStatusUpdate(newBastion, _ *operations.Bastion) field.ErrorList {
	allErrs := field.ErrorList{}
	now := time.Now()

	if newBastion.Status.LastHeartbeatTimestamp.After(now) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status.lastHeartbeatTimestamp"), newBastion.Status.LastHeartbeatTimestamp, "last heartbeat must not be in the future"))
	}

	return allErrs
}
