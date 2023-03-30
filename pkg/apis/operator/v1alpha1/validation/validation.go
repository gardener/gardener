// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation

import (
	"fmt"
	"net"

	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencoreinstall "github.com/gardener/gardener/pkg/apis/core/install"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
)

var gardenCoreScheme *runtime.Scheme

func init() {
	gardenCoreScheme = runtime.NewScheme()
	utilruntime.Must(gardencoreinstall.AddToScheme(gardenCoreScheme))
}

// ValidateGarden contains functionality for performing extended validation of a Garden object which is not possible
// with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGarden(garden *operatorv1alpha1.Garden) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateOperation(garden.Annotations[v1beta1constants.GardenerOperation], garden, field.NewPath("metadata", "annotations"))...)
	allErrs = append(allErrs, validateVirtualCluster(garden.Spec.VirtualCluster, field.NewPath("spec", "virtualCluster"))...)

	if helper.TopologyAwareRoutingEnabled(garden.Spec.RuntimeCluster.Settings) {
		if len(garden.Spec.RuntimeCluster.Provider.Zones) <= 1 {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "runtimeCluster", "settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled on multi-zone garden runtime cluster (with at least two zones in spec.provider.zones)"))
		}
		if !helper.HighAvailabilityEnabled(garden) {
			allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "runtimeCluster", "settings", "topologyAwareRouting", "enabled"), "topology-aware routing can only be enabled when virtual cluster's high-availability is enabled"))
		}
	}

	return allErrs
}

// ValidateGardenUpdate contains functionality for performing extended validation of a Garden object under update which
// is not possible with standard CRD validation, see https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definitions/#validation-rules.
func ValidateGardenUpdate(oldGarden, newGarden *operatorv1alpha1.Garden) field.ErrorList {
	allErrs := field.ErrorList{}

	if oldGarden.Spec.VirtualCluster.ControlPlane != nil && oldGarden.Spec.VirtualCluster.ControlPlane.HighAvailability != nil &&
		(newGarden.Spec.VirtualCluster.ControlPlane == nil || newGarden.Spec.VirtualCluster.ControlPlane.HighAvailability == nil) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(oldGarden.Spec.VirtualCluster.ControlPlane, newGarden.Spec.VirtualCluster.ControlPlane, field.NewPath("spec", "virtualCluster", "controlPlane", "highAvailability"))...)
	}

	allErrs = append(allErrs, gardencorevalidation.ValidateKubernetesVersionUpdate(newGarden.Spec.VirtualCluster.Kubernetes.Version, newGarden.Spec.VirtualCluster.Kubernetes.Version, field.NewPath("spec", "virtualCluster", "kubernetes", "version"))...)
	allErrs = append(allErrs, ValidateGarden(newGarden)...)

	return allErrs
}

func validateVirtualCluster(virtualCluster operatorv1alpha1.VirtualCluster, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, gardencorevalidation.ValidateDNS1123Subdomain(virtualCluster.DNS.Domain, fldPath.Child("dns", "domain"))...)

	if err := kubernetesversion.CheckIfSupported(virtualCluster.Kubernetes.Version); err != nil {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kubernetes", "version"), virtualCluster.Kubernetes.Version, kubernetesversion.SupportedVersions))
	}

	if kubeAPIServer := virtualCluster.Kubernetes.KubeAPIServer; kubeAPIServer != nil && kubeAPIServer.KubeAPIServerConfig != nil {
		path := fldPath.Child("kubernetes", "kubeAPIServer")

		coreKubeAPIServerConfig := &gardencore.KubeAPIServerConfig{}
		if err := gardenCoreScheme.Convert(kubeAPIServer.KubeAPIServerConfig, coreKubeAPIServerConfig, nil); err != nil {
			allErrs = append(allErrs, field.InternalError(path, err))
		}

		allErrs = append(allErrs, gardencorevalidation.ValidateKubeAPIServer(coreKubeAPIServerConfig, virtualCluster.Kubernetes.Version, true, path)...)
	}

	if _, _, err := net.ParseCIDR(virtualCluster.Networking.Services); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("networking", "services"), virtualCluster.Networking.Services, fmt.Sprintf("cannot parse service network cidr: %s", err.Error())))
	}

	return allErrs
}

func validateOperation(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if operation == "" {
		return allErrs
	}

	fldPathOp := fldPath.Key(v1beta1constants.GardenerOperation)

	if operation != "" && !operatorv1alpha1.AvailableOperationAnnotations.Has(operation) {
		allErrs = append(allErrs, field.NotSupported(fldPathOp, operation, sets.List(operatorv1alpha1.AvailableOperationAnnotations)))
	}
	allErrs = append(allErrs, validateOperationContext(operation, garden, fldPathOp)...)

	return allErrs
}

func validateOperationContext(operation string, garden *operatorv1alpha1.Garden, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch operation {
	case v1beta1constants.OperationRotateCredentialsStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if garden has deletion timestamp"))
		}
		if phase := helper.GetCARotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
		if phase := helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.serviceAccountKey.phase is not 'Completed'"))
		}
		if phase := helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateCredentialsComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if garden has deletion timestamp"))
		}
		if helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}
		if helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.serviceAccountKey.phase is not 'Prepared'"))
		}
		if helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateCAStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetCARotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateCAComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if garden has deletion timestamp"))
		}
		if helper.GetCARotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateServiceAccountKeyStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start service account key rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start service account key rotation if .status.credentials.rotation.serviceAccountKey.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateServiceAccountKeyComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete service account key rotation if garden has deletion timestamp"))
		}
		if helper.GetServiceAccountKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete service account key rotation if .status.credentials.rotation.serviceAccountKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateETCDEncryptionKeyStart:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if garden has deletion timestamp"))
		}
		if phase := helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials); len(phase) > 0 && phase != gardencorev1beta1.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateETCDEncryptionKeyComplete:
		if garden.DeletionTimestamp != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete ETCD encryption key rotation if garden has deletion timestamp"))
		}
		if helper.GetETCDEncryptionKeyRotationPhase(garden.Status.Credentials) != gardencorev1beta1.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Prepared'"))
		}
	}

	return allErrs
}
