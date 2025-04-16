// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"math/big"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-test/deep"
	"github.com/robfig/cron"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/timewindow"
	admissionpluginsvalidation "github.com/gardener/gardener/pkg/utils/validation/admissionplugins"
	apigroupsvalidation "github.com/gardener/gardener/pkg/utils/validation/apigroups"
	cidrvalidation "github.com/gardener/gardener/pkg/utils/validation/cidr"
	featuresvalidation "github.com/gardener/gardener/pkg/utils/validation/features"
	kubernetescorevalidation "github.com/gardener/gardener/pkg/utils/validation/kubernetes/core"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

var (
	availableProxyModes = sets.New(
		string(core.ProxyModeIPTables),
		string(core.ProxyModeIPVS),
	)
	availableKubernetesDashboardAuthenticationModes = sets.New(
		core.KubernetesDashboardAuthModeToken,
	)
	availableNginxIngressExternalTrafficPolicies = sets.New(
		string(corev1.ServiceExternalTrafficPolicyCluster),
		string(corev1.ServiceExternalTrafficPolicyLocal),
	)
	availableShootOperations = sets.New(
		v1beta1constants.ShootOperationMaintain,
		v1beta1constants.ShootOperationRetry,
		v1beta1constants.ShootOperationForceInPlaceUpdate,
	).Union(availableShootMaintenanceOperations)
	availableShootMaintenanceOperations = sets.New(
		v1beta1constants.GardenerOperationReconcile,
		v1beta1constants.OperationRotateCAStart,
		v1beta1constants.OperationRotateCAStartWithoutWorkersRollout,
		v1beta1constants.OperationRotateCAComplete,
		v1beta1constants.OperationRotateObservabilityCredentials,
		v1beta1constants.ShootOperationRotateSSHKeypair,
		v1beta1constants.OperationRotateRolloutWorkers,
	).Union(forbiddenShootOperationsWhenHibernated)
	forbiddenShootOperationsWhenHibernated = sets.New(
		v1beta1constants.OperationRotateCredentialsStart,
		v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
		v1beta1constants.OperationRotateCredentialsComplete,
		v1beta1constants.OperationRotateETCDEncryptionKeyStart,
		v1beta1constants.OperationRotateETCDEncryptionKeyComplete,
		v1beta1constants.OperationRotateServiceAccountKeyStart,
		v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout,
		v1beta1constants.OperationRotateServiceAccountKeyComplete,
	)
	forbiddenShootOperationsWhenEncryptionChangeIsRollingOut = sets.New(
		v1beta1constants.OperationRotateCredentialsStart,
		v1beta1constants.OperationRotateETCDEncryptionKeyStart,
	)
	availableShootPurposes = sets.New(
		string(core.ShootPurposeEvaluation),
		string(core.ShootPurposeTesting),
		string(core.ShootPurposeDevelopment),
		string(core.ShootPurposeProduction),
	)
	availableWorkerCRINames = sets.New(
		string(core.CRINameContainerD),
	)
	availableClusterAutoscalerExpanderModes = sets.New(
		string(core.ClusterAutoscalerExpanderLeastWaste),
		string(core.ClusterAutoscalerExpanderMostPods),
		string(core.ClusterAutoscalerExpanderPriority),
		string(core.ClusterAutoscalerExpanderRandom),
	)
	availableCoreDNSAutoscalingModes = sets.New(
		string(core.CoreDNSAutoscalingModeClusterProportional),
		string(core.CoreDNSAutoscalingModeHorizontal),
	)
	availableSchedulingProfiles = sets.New(
		string(core.SchedulingProfileBalanced),
		string(core.SchedulingProfileBinPacking),
	)
	errorCodesAllowingForceDeletion = sets.New(
		core.ErrorInfraUnauthenticated,
		core.ErrorInfraUnauthorized,
		core.ErrorInfraDependencies,
		core.ErrorCleanupClusterResources,
		core.ErrorConfigurationProblem,
	)
	// ForbiddenShootFinalizersOnCreation is a list of finalizers which are forbidden to be specified on Shoot creation.
	ForbiddenShootFinalizersOnCreation = sets.New(
		gardencorev1beta1.GardenerName,
		v1beta1constants.ReferenceProtectionFinalizerName,
	)
	availableUpdateStrategies = sets.New(core.AutoRollingUpdate, core.AutoInPlaceUpdate, core.ManualInPlaceUpdate)

	// asymmetric algorithms from https://datatracker.ietf.org/doc/html/rfc7518#section-3.1
	availableOIDCSigningAlgs = sets.New(
		"RS256",
		"RS384",
		"RS512",
		"ES256",
		"ES384",
		"ES512",
		"PS256",
		"PS384",
		"PS512",
		"none",
	)

	workerlessErrorMsg = "this field should not be set for workerless Shoot clusters"
)

// ValidateShoot validates a Shoot object.
func ValidateShoot(shoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMeta(&shoot.ObjectMeta, true, apivalidation.NameIsDNSLabel, field.NewPath("metadata"))...)
	allErrs = append(allErrs, validateNameConsecutiveHyphens(shoot.Name, field.NewPath("metadata", "name"))...)
	allErrs = append(allErrs, validateShootOperation(shoot.Annotations[v1beta1constants.GardenerOperation], shoot.Annotations[v1beta1constants.GardenerMaintenanceOperation], shoot, field.NewPath("metadata", "annotations"))...)
	allErrs = append(allErrs, ValidateShootSpec(shoot.ObjectMeta, &shoot.Spec, field.NewPath("spec"), false)...)
	allErrs = append(allErrs, ValidateShootHAConfig(shoot)...)

	return allErrs
}

// ValidateShootUpdate validates a Shoot object before an update.
func ValidateShootUpdate(newShoot, oldShoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateObjectMetaUpdate(&newShoot.ObjectMeta, &oldShoot.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootObjectMetaUpdate(newShoot.ObjectMeta, oldShoot.ObjectMeta, field.NewPath("metadata"))...)
	allErrs = append(allErrs, ValidateShootSpecUpdate(&newShoot.Spec, &oldShoot.Spec, newShoot.ObjectMeta, field.NewPath("spec"))...)

	var (
		etcdEncryptionKeyRotation *core.ETCDEncryptionKeyRotation
		oldEncryptionConfig       *core.EncryptionConfig
		newEncryptionConfig       *core.EncryptionConfig
		hibernationEnabled        = false
	)

	if credentials := newShoot.Status.Credentials; credentials != nil && credentials.Rotation != nil {
		etcdEncryptionKeyRotation = credentials.Rotation.ETCDEncryptionKey
	}
	if oldShoot.Spec.Kubernetes.KubeAPIServer != nil {
		oldEncryptionConfig = oldShoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig
	}
	if newShoot.Spec.Kubernetes.KubeAPIServer != nil {
		newEncryptionConfig = newShoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig
	}
	if newShoot.Spec.Hibernation != nil {
		hibernationEnabled = ptr.Deref(newShoot.Spec.Hibernation.Enabled, false)
	}

	allErrs = append(allErrs, ValidateEncryptionConfigUpdate(newEncryptionConfig, oldEncryptionConfig, sets.New(newShoot.Status.EncryptedResources...), etcdEncryptionKeyRotation, hibernationEnabled, field.NewPath("spec", "kubernetes", "kubeAPIServer", "encryptionConfig"))...)
	allErrs = append(allErrs, ValidateShoot(newShoot)...)
	allErrs = append(allErrs, ValidateShootHAConfigUpdate(newShoot, oldShoot)...)
	allErrs = append(allErrs, validateHibernationUpdate(newShoot, oldShoot)...)
	allErrs = append(allErrs, ValidateForceDeletion(newShoot, oldShoot)...)
	allErrs = append(allErrs, validateNodeLocalDNSUpdate(&newShoot.Spec, &oldShoot.Spec, field.NewPath("spec"))...)
	allErrs = append(allErrs, ValidateInPlaceUpdates(newShoot, oldShoot)...)

	return allErrs
}

// ValidateShootTemplate validates a ShootTemplate.
func ValidateShootTemplate(shootTemplate *core.ShootTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, metav1validation.ValidateLabels(shootTemplate.Labels, fldPath.Child("metadata", "labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(shootTemplate.Annotations, fldPath.Child("metadata", "annotations"))...)
	allErrs = append(allErrs, ValidateShootSpec(shootTemplate.ObjectMeta, &shootTemplate.Spec, fldPath.Child("spec"), true)...)

	return allErrs
}

// ValidateShootTemplateUpdate validates a ShootTemplate before an update.
func ValidateShootTemplateUpdate(newShootTemplate, oldShootTemplate *core.ShootTemplate, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateShootSpecUpdate(&newShootTemplate.Spec, &oldShootTemplate.Spec, newShootTemplate.ObjectMeta, fldPath.Child("spec"))...)

	if oldShootTemplate.Spec.Networking != nil && oldShootTemplate.Spec.Networking.Nodes != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newShootTemplate.Spec.Networking.Nodes, oldShootTemplate.Spec.Networking.Nodes, fldPath.Child("spec", "networking", "nodes"))...)
	}

	return allErrs
}

// ValidateShootObjectMetaUpdate validates the object metadata of a Shoot object.
func ValidateShootObjectMetaUpdate(_, _ metav1.ObjectMeta, _ *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	return allErrs
}

// ValidateShootSpec validates the specification of a Shoot object.
func ValidateShootSpec(meta metav1.ObjectMeta, spec *core.ShootSpec, fldPath *field.Path, inTemplate bool) field.ErrorList {
	var (
		allErrs    = field.ErrorList{}
		workerless = len(spec.Provider.Workers) == 0
	)

	allErrs = append(allErrs, ValidateCloudProfileReference(spec.CloudProfile, spec.CloudProfileName, fldPath.Child("cloudProfile"))...)
	allErrs = append(allErrs, validateProvider(spec.Provider, spec.Kubernetes, spec.Networking, workerless, fldPath.Child("provider"), inTemplate)...)
	allErrs = append(allErrs, validateAddons(spec.Addons, spec.Purpose, workerless, fldPath.Child("addons"))...)
	allErrs = append(allErrs, validateDNS(spec.DNS, fldPath.Child("dns"))...)
	allErrs = append(allErrs, validateExtensions(spec.Extensions, fldPath.Child("extensions"))...)
	allErrs = append(allErrs, validateResources(spec.Resources, fldPath.Child("resources"))...)
	allErrs = append(allErrs, validateKubernetes(spec.Kubernetes, spec.Networking, workerless, fldPath.Child("kubernetes"))...)
	allErrs = append(allErrs, validateNetworking(spec.Networking, workerless, fldPath.Child("networking"))...)
	allErrs = append(allErrs, validateMaintenance(spec.Maintenance, fldPath.Child("maintenance"), workerless)...)
	allErrs = append(allErrs, validateMonitoring(spec.Monitoring, fldPath.Child("monitoring"))...)
	allErrs = append(allErrs, ValidateHibernation(meta.Annotations, spec.Hibernation, fldPath.Child("hibernation"))...)

	if len(spec.Region) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("region"), "must specify a region"))
	}
	if workerless {
		if spec.SecretBindingName != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("secretBindingName"), workerlessErrorMsg))
		}
		if spec.CredentialsBindingName != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("credentialsBindingName"), workerlessErrorMsg))
		}
	} else {
		if len(ptr.Deref(spec.SecretBindingName, "")) == 0 && len(ptr.Deref(spec.CredentialsBindingName, "")) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("secretBindingName"), "must be set when credentialsBindingName is not"))
		} else if len(ptr.Deref(spec.SecretBindingName, "")) != 0 && len(ptr.Deref(spec.CredentialsBindingName, "")) != 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("secretBindingName"), "is incompatible with credentialsBindingName"))
		}
	}
	if spec.SeedName != nil && len(*spec.SeedName) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("seedName"), spec.SeedName, "seed name must not be empty when providing the key"))
	}
	if spec.SeedSelector != nil {
		allErrs = append(allErrs, metav1validation.ValidateLabelSelector(&spec.SeedSelector.LabelSelector, metav1validation.LabelSelectorValidationOptions{AllowInvalidLabelValueInSelector: true}, fldPath.Child("seedSelector"))...)
	}
	if purpose := spec.Purpose; purpose != nil {
		allowedShootPurposes := availableShootPurposes
		if meta.Namespace == v1beta1constants.GardenNamespace || inTemplate {
			allowedShootPurposes = sets.New(append(sets.List(availableShootPurposes), string(core.ShootPurposeInfrastructure))...)
		}

		if !allowedShootPurposes.Has(string(*purpose)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("purpose"), *purpose, sets.List(allowedShootPurposes)))
		}
	}
	allErrs = append(allErrs, ValidateTolerations(spec.Tolerations, fldPath.Child("tolerations"))...)
	allErrs = append(allErrs, ValidateSystemComponents(spec.SystemComponents, fldPath.Child("systemComponents"), workerless)...)

	return allErrs
}

// ValidateShootSpecUpdate validates the specification of a Shoot object.
func ValidateShootSpecUpdate(newSpec, oldSpec *core.ShootSpec, newObjectMeta metav1.ObjectMeta, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if newObjectMeta.DeletionTimestamp != nil && !apiequality.Semantic.DeepEqual(newSpec, oldSpec) {
		if diff := deep.Equal(newSpec, oldSpec); diff != nil {
			return field.ErrorList{field.Forbidden(fldPath, strings.Join(diff, ","))}
		}
		return apivalidation.ValidateImmutableField(newSpec, oldSpec, fldPath)
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.Region, oldSpec.Region, fldPath.Child("region"))...)
	allErrs = append(allErrs, ValidateCloudProfileReference(newSpec.CloudProfile, newSpec.CloudProfileName, fldPath.Child("cloudProfile"))...)

	if oldSpec.CredentialsBindingName != nil && len(ptr.Deref(newSpec.CredentialsBindingName, "")) == 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("credentialsBindingName"), "the field cannot be unset"))
	}

	// allow removing the value of SecretBindingName when
	// old secret binding existed, but new is set to nil
	// and new credentials binding also exists
	migrationFromSecBindingToCredBinding := oldSpec.SecretBindingName != nil && newSpec.SecretBindingName == nil && len(ptr.Deref(newSpec.CredentialsBindingName, "")) > 0
	if !migrationFromSecBindingToCredBinding {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.SecretBindingName, oldSpec.SecretBindingName, fldPath.Child("secretBindingName"))...)
	}
	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.ExposureClassName, oldSpec.ExposureClassName, fldPath.Child("exposureClassName"))...)

	allErrs = append(allErrs, validateDNSUpdate(newSpec.DNS, oldSpec.DNS, newSpec.SeedName != nil, fldPath.Child("dns"))...)
	allErrs = append(allErrs, ValidateKubernetesVersionUpdate(newSpec.Kubernetes.Version, oldSpec.Kubernetes.Version, false, fldPath.Child("kubernetes", "version"))...)

	allErrs = append(allErrs, validateKubeControllerManagerUpdate(newSpec.Kubernetes.KubeControllerManager, oldSpec.Kubernetes.KubeControllerManager, fldPath.Child("kubernetes", "kubeControllerManager"))...)

	if newSpec.SeedName != nil {
		if oldSpec.SeedSelector == nil && newSpec.SeedSelector != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("seedSelector"), "cannot set seed selector when .spec.seedName is set"))
		}
		if oldSpec.SeedSelector != nil && newSpec.SeedSelector != nil {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(newSpec.SeedSelector, oldSpec.SeedSelector, fldPath.Child("seedSelector"))...)
		}
	}

	if err := validateWorkerUpdate(len(newSpec.Provider.Workers) > 0, len(oldSpec.Provider.Workers) > 0, fldPath.Child("provider", "workers")); err != nil {
		allErrs = append(allErrs, err)
		return allErrs
	}

	allErrs = append(allErrs, ValidateProviderUpdate(&newSpec.Provider, &oldSpec.Provider, fldPath.Child("provider"))...)

	for i, newWorker := range newSpec.Provider.Workers {
		oldWorker := newWorker
		for _, ow := range oldSpec.Provider.Workers {
			if ow.Name == newWorker.Name {
				oldWorker = ow
				break
			}
		}
		idxPath := fldPath.Child("provider", "workers").Index(i)

		oldKubernetesVersion := oldSpec.Kubernetes.Version
		newKubernetesVersion := newSpec.Kubernetes.Version
		if oldWorker.Kubernetes != nil && oldWorker.Kubernetes.Version != nil {
			oldKubernetesVersion = *oldWorker.Kubernetes.Version
		}
		if newWorker.Kubernetes != nil && newWorker.Kubernetes.Version != nil {
			newKubernetesVersion = *newWorker.Kubernetes.Version
		}

		// worker Kubernetes versions must not be downgraded; minor version skips are allowed, except for AutoInPlaceUpdate and ManualInPlaceUpdate.
		allErrs = append(allErrs, ValidateKubernetesVersionUpdate(newKubernetesVersion, oldKubernetesVersion, !helper.IsUpdateStrategyInPlace(newWorker.UpdateStrategy), idxPath.Child("kubernetes", "version"))...)
	}

	allErrs = append(allErrs, validateNetworkingUpdate(newSpec.Networking, oldSpec.Networking, fldPath.Child("networking"))...)

	if !reflect.DeepEqual(oldSpec.SchedulerName, newSpec.SchedulerName) {
		// only allow to set an empty scheduler name to the default scheduler
		if oldSpec.SchedulerName != nil || ptr.Deref(newSpec.SchedulerName, "") != v1beta1constants.DefaultSchedulerName {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("schedulerName"), newSpec.SchedulerName, "field is immutable"))
		}
	}

	return allErrs
}

func validateWorkerUpdate(newHasWorkers, oldHasWorkers bool, fldPath *field.Path) *field.Error {
	if oldHasWorkers && !newHasWorkers {
		return field.Forbidden(fldPath, "cannot switch from a Shoot with workers to a workerless Shoot")
	}
	if !oldHasWorkers && newHasWorkers {
		return field.Forbidden(fldPath, "cannot switch from a workerless Shoot to a Shoot with workers")
	}

	return nil
}

// ValidateProviderUpdate validates the specification of a Provider object.
func ValidateProviderUpdate(newProvider, oldProvider *core.Provider, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newProvider.Type, oldProvider.Type, fldPath.Child("type"))...)

	for i, newWorker := range newProvider.Workers {
		var oldWorker core.Worker

		oldWorkerIndex := slices.IndexFunc(oldProvider.Workers, func(worker core.Worker) bool {
			oldWorker = worker
			return worker.Name == newWorker.Name
		})

		if oldWorkerIndex == -1 {
			continue
		}

		var (
			idxPath              = fldPath.Child("workers").Index(i)
			oldStrategyIsInPlace = helper.IsUpdateStrategyInPlace(oldWorker.UpdateStrategy)
			newStrategyIsInPlace = helper.IsUpdateStrategyInPlace(newWorker.UpdateStrategy)
		)

		if oldStrategyIsInPlace && !newStrategyIsInPlace {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("updateStrategy"), newWorker.UpdateStrategy, "update strategy cannot be changed from AutoInPlaceUpdate/ManualInPlaceUpdate to AutoRollingUpdate"))
		}

		if !oldStrategyIsInPlace && newStrategyIsInPlace {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("updateStrategy"), newWorker.UpdateStrategy, "update strategy cannot be changed from AutoRollingUpdate to AutoInPlaceUpdate/ManualInPlaceUpdate"))
		}

		if ptr.Equal(oldWorker.UpdateStrategy, newWorker.UpdateStrategy) && helper.IsUpdateStrategyInPlace(newWorker.UpdateStrategy) {
			if oldWorker.Machine.Type != newWorker.Machine.Type {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("machine", "type"), newWorker.Machine.Type, "machine type cannot be changed if update strategy is AutoInPlaceUpdate/ManualInPlaceUpdate"))
			}

			if oldWorker.Machine.Image != nil && newWorker.Machine.Image != nil && oldWorker.Machine.Image.Name != newWorker.Machine.Image.Name {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("machine", "image", "name"), newWorker.Machine.Image.Name, "machine image name cannot be changed if update strategy is AutoInPlaceUpdate/ManualInPlaceUpdate"))
			}

			if oldWorker.CRI != nil && newWorker.CRI != nil && oldWorker.CRI.Name != newWorker.CRI.Name {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("cri", "name"), newWorker.CRI.Name, "CRI name cannot be changed if update strategy is AutoInPlaceUpdate/ManualInPlaceUpdate"))
			}

			if !apiequality.Semantic.DeepEqual(oldWorker.Volume, newWorker.Volume) {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("volume"), newWorker.Volume, "volume cannot be changed if update strategy is AutoInPlaceUpdate/ManualInPlaceUpdate"))
			}
		}
	}

	return allErrs
}

// ValidateShootStatusUpdate validates the status field of a Shoot object.
func ValidateShootStatusUpdate(newStatus, oldStatus core.ShootStatus) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		fldPath = field.NewPath("status")
	)

	if len(oldStatus.UID) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newStatus.UID, oldStatus.UID, fldPath.Child("uid"))...)
	}
	if len(oldStatus.TechnicalID) > 0 {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newStatus.TechnicalID, oldStatus.TechnicalID, fldPath.Child("technicalID"))...)
	}

	if oldStatus.ClusterIdentity != nil && !apiequality.Semantic.DeepEqual(oldStatus.ClusterIdentity, newStatus.ClusterIdentity) {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newStatus.ClusterIdentity, oldStatus.ClusterIdentity, fldPath.Child("clusterIdentity"))...)
	}
	if len(newStatus.AdvertisedAddresses) > 0 {
		allErrs = append(allErrs, validateAdvertiseAddresses(newStatus.AdvertisedAddresses, fldPath.Child("advertisedAddresses"))...)
	}

	allErrs = append(allErrs, validateNetworkingStatus(newStatus.Networking, fldPath.Child("networking"))...)

	return allErrs
}

// validateAdvertiseAddresses validates kube-apiserver addresses.
func validateAdvertiseAddresses(addresses []core.ShootAdvertisedAddress, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	names := sets.New[string]()
	for i, address := range addresses {
		if address.Name == "" {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("name"), "field must not be empty"))
		} else if names.Has(address.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), address.Name))
		} else {
			names.Insert(address.Name)
			allErrs = append(allErrs, validateAdvertisedURL(address.URL, fldPath.Index(i).Child("url"))...)
		}
	}
	return allErrs
}

// validateAdvertisedURL validates kube-apiserver's URL.
func validateAdvertisedURL(URL string, fldPath *field.Path) field.ErrorList {
	var allErrors field.ErrorList
	const form = "; desired format: https://host[:port]"
	if u, err := url.Parse(URL); err != nil {
		allErrors = append(allErrors, field.Required(fldPath, "url must be a valid URL: "+err.Error()+form))
	} else {
		if u.Scheme != "https" {
			allErrors = append(allErrors, field.Invalid(fldPath, u.Scheme, "'https' is the only allowed URL scheme"+form))
		}
		if len(u.Host) == 0 {
			allErrors = append(allErrors, field.Invalid(fldPath, u.Host, "host must be provided"+form))
		}
		if u.User != nil {
			allErrors = append(allErrors, field.Invalid(fldPath, u.User.String(), "user information is not permitted in the URL"+form))
		}
		if len(u.Fragment) != 0 {
			allErrors = append(allErrors, field.Invalid(fldPath, u.Fragment, "fragments are not permitted in the URL"+form))
		}
		if len(u.RawQuery) != 0 {
			allErrors = append(allErrors, field.Invalid(fldPath, u.RawQuery, "query parameters are not permitted in the URL"+form))
		}
	}
	return allErrors
}

func validateAddons(addons *core.Addons, purpose *core.ShootPurpose, workerless bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if workerless && addons != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath, "addons cannot be enabled for Workerless Shoot clusters"))
		return allErrs
	}

	if (helper.NginxIngressEnabled(addons) || helper.KubernetesDashboardEnabled(addons)) && (purpose != nil && *purpose != core.ShootPurposeEvaluation) {
		allErrs = append(allErrs, field.Forbidden(fldPath, "addons can only be enabled on shoots with .spec.purpose=evaluation"))
	}

	if helper.NginxIngressEnabled(addons) {
		if policy := addons.NginxIngress.ExternalTrafficPolicy; policy != nil {
			if !availableNginxIngressExternalTrafficPolicies.Has(string(*policy)) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("nginxIngress", "externalTrafficPolicy"), *policy, sets.List(availableNginxIngressExternalTrafficPolicies)))
			}
		}
	}

	if helper.KubernetesDashboardEnabled(addons) {
		if authMode := addons.KubernetesDashboard.AuthenticationMode; authMode != nil {
			if !availableKubernetesDashboardAuthenticationModes.Has(*authMode) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("kubernetesDashboard", "authenticationMode"), *authMode, sets.List(availableKubernetesDashboardAuthenticationModes)))
			}
		}
	}

	return allErrs
}

const (
	// kube-controller-manager's default value for --node-cidr-mask-size for IPv4
	defaultNodeCIDRMaskSizeV4 = 24
	// kube-controller-manager's default value for --node-cidr-mask-size for IPv6
	defaultNodeCIDRMaskSizeV6 = 64
)

// ValidateNodeCIDRMaskWithMaxPod validates if the Pod Network has enough ip addresses (configured via the NodeCIDRMask on the kube controller manager) to support the highest max pod setting on the shoot
func ValidateNodeCIDRMaskWithMaxPod(maxPod int32, nodeCIDRMaskSize int32, networking core.Networking) field.ErrorList {
	allErrs := field.ErrorList{}

	totalBitLen := int32(net.IPv4len * 8) // entire IPv4 bit length
	defaultNodeCIDRMaskSize := defaultNodeCIDRMaskSizeV4

	if core.IsIPv6SingleStack(networking.IPFamilies) {
		totalBitLen = net.IPv6len * 8 // entire IPv6 bit length
		defaultNodeCIDRMaskSize = defaultNodeCIDRMaskSizeV6
	}

	// Each Node gets assigned a subnet of the entire pod network with a mask size of nodeCIDRMaskSize,
	// calculate bit length of a single podCIDR subnet (Node.status.podCIDR).
	subnetBitLen := totalBitLen - nodeCIDRMaskSize

	// Calculate how many addresses a single podCIDR subnet contains.
	// This will overflow uint64 if nodeCIDRMaskSize <= 64 (subnetBitLen >= 64, default in IPv6), so use big.Int
	ipAddressesAvailable := &big.Int{}
	ipAddressesAvailable.Exp(big.NewInt(2), big.NewInt(int64(subnetBitLen)), nil)
	// first and last ips are reserved, subtract 2
	ipAddressesAvailable.Sub(ipAddressesAvailable, big.NewInt(2))

	if ipAddressesAvailable.Cmp(big.NewInt(int64(maxPod))) < 0 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec").Child("kubernetes").Child("kubeControllerManager").Child("nodeCIDRMaskSize"), nodeCIDRMaskSize, fmt.Sprintf("kubelet or kube-controller-manager configuration incorrect. Please adjust the nodeCIDRMaskSize to support the highest maxPod on any worker pool. The nodeCIDRMaskSize of %d (default: %d) only supports %d IP addresses. The highest maxPod setting is %d (default: 110). Please choose a nodeCIDRMaskSize that at least supports %d IP addresses", nodeCIDRMaskSize, defaultNodeCIDRMaskSize, ipAddressesAvailable, maxPod, maxPod)))
	}

	return allErrs
}

// ValidateEncryptionConfigUpdate validates the updates to the KubeAPIServer encryption configuration.
func ValidateEncryptionConfigUpdate(newConfig, oldConfig *core.EncryptionConfig, currentEncryptedResources sets.Set[string], etcdEncryptionKeyRotation *core.ETCDEncryptionKeyRotation, isClusterInHibernation bool, fldPath *field.Path) field.ErrorList {
	var (
		allErrs               = field.ErrorList{}
		oldEncryptedResources = sets.New[string]()
		newEncryptedResources = sets.New[string]()
	)

	if oldConfig != nil {
		oldEncryptedResources.Insert(oldConfig.Resources...)
	}

	if newConfig != nil {
		newEncryptedResources.Insert(newConfig.Resources...)
	}

	if !newEncryptedResources.Equal(oldEncryptedResources) {
		if etcdEncryptionKeyRotation != nil && etcdEncryptionKeyRotation.Phase != core.RotationCompleted && etcdEncryptionKeyRotation.Phase != "" {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("resources"), fmt.Sprintf("resources cannot be changed when .status.credentials.rotation.etcdEncryptionKey.phase is not %q", string(core.RotationCompleted))))
		}

		if !oldEncryptedResources.Equal(currentEncryptedResources) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("resources"), "resources cannot be changed because a previous encryption configuration change is currently being rolled out"))
		}

		if isClusterInHibernation {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("resources"), "resources cannot be changed when shoot is in hibernation"))
		}
	}

	return allErrs
}

func validateKubeControllerManagerUpdate(newConfig, oldConfig *core.KubeControllerManagerConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	var (
		nodeCIDRMaskNew *int32
		nodeCIDRMaskOld *int32
	)

	if newConfig != nil {
		nodeCIDRMaskNew = newConfig.NodeCIDRMaskSize
	}
	if oldConfig != nil {
		nodeCIDRMaskOld = oldConfig.NodeCIDRMaskSize
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(nodeCIDRMaskNew, nodeCIDRMaskOld, fldPath.Child("nodeCIDRMaskSize"))...)

	return allErrs
}

func validateDNSUpdate(new, old *core.DNS, seedGotAssigned bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if old != nil && new == nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(new, old, fldPath)...)
	}

	if new != nil && old != nil {
		if old.Domain != nil && new.Domain != old.Domain {
			allErrs = append(allErrs, apivalidation.ValidateImmutableField(new.Domain, old.Domain, fldPath.Child("domain"))...)
		}

		if seedGotAssigned {
			var (
				primaryOld = helper.FindPrimaryDNSProvider(old.Providers)
				primaryNew = helper.FindPrimaryDNSProvider(new.Providers)
			)

			if primaryOld != nil && primaryNew == nil {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("providers"), "removing a primary provider is not allowed"))
			}

			if primaryOld != nil && primaryOld.Type != nil && primaryNew != nil && primaryNew.Type == nil {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("providers"), "removing the primary provider type is not allowed"))
			}

			if primaryOld != nil && primaryOld.Type != nil && primaryNew != nil && primaryNew.Type != nil && *primaryOld.Type != *primaryNew.Type {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("providers"), "changing primary provider type is not allowed"))
			}
		}
	}

	return allErrs
}

// ValidateKubernetesVersionUpdate ensures that new version is newer than old version and does not skip minor versions when not allowed
func ValidateKubernetesVersionUpdate(new, old string, skipMinorVersionAllowed bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(new) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, new, "cannot validate kubernetes version upgrade because it is unset"))
		return allErrs
	}

	// Forbid Kubernetes version downgrade
	downgrade, err := versionutils.CompareVersions(new, "<", old)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, new, err.Error()))
	}
	if downgrade {
		allErrs = append(allErrs, field.Forbidden(fldPath, "kubernetes version downgrade is not supported"))
	}

	if !skipMinorVersionAllowed {
		// Forbid Kubernetes version upgrade which skips a minor version
		oldVersion, err := semver.NewVersion(old)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, old, err.Error()))
		}
		nextMinorVersion := oldVersion.IncMinor().IncMinor()

		skippingMinorVersion, err := versionutils.CompareVersions(new, ">=", nextMinorVersion.String())
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, new, err.Error()))
		}
		if skippingMinorVersion {
			allErrs = append(allErrs, field.Forbidden(fldPath, "kubernetes version upgrade cannot skip a minor version"))
		}
	}

	return allErrs
}

func validateNodeLocalDNSUpdate(newSpec, oldSpec *core.ShootSpec, fldPath *field.Path) field.ErrorList {
	var (
		allErrs                = field.ErrorList{}
		oldNodeLocalDNSEnabled = oldSpec.SystemComponents != nil && oldSpec.SystemComponents.NodeLocalDNS != nil && oldSpec.SystemComponents.NodeLocalDNS.Enabled
		newNodeLocalDNSEnabled = newSpec.SystemComponents != nil && newSpec.SystemComponents.NodeLocalDNS != nil && newSpec.SystemComponents.NodeLocalDNS.Enabled
	)

	if oldNodeLocalDNSEnabled != newNodeLocalDNSEnabled {
		for _, worker := range oldSpec.Provider.Workers {
			if helper.IsUpdateStrategyInPlace(worker.UpdateStrategy) {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("systemComponents", "nodeLocalDNS"), "node-local-dns setting can not be changed if shoot has at least one worker pool with update strategy AutoInPlaceUpdate/ManualInPlaceUpdate"))
				break
			}
		}
	}

	return allErrs
}

func validateNetworkingUpdate(newNetworking, oldNetworking *core.Networking, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if oldNetworking == nil {
		// if the old networking is nil, we cannot validate immutability anyway, so exit early
		return allErrs
	}
	if newNetworking == nil {
		allErrs = append(allErrs, field.Forbidden(fldPath, "networking cannot be set to nil if it's already set"))
		return allErrs
	}

	allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworking.Type, oldNetworking.Type, fldPath.Child("type"))...)
	if oldNetworking.Pods != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworking.Pods, oldNetworking.Pods, fldPath.Child("pods"))...)
	}
	if oldNetworking.Services != nil {
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newNetworking.Services, oldNetworking.Services, fldPath.Child("services"))...)
	}

	return allErrs
}

// validateWorkerGroupAndControlPlaneKubernetesVersion ensures that new version is newer than old version and does not skip two minor
func validateWorkerGroupAndControlPlaneKubernetesVersion(controlPlaneVersion, workerGroupVersion string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// worker group kubernetes version must not be higher than controlplane version
	uplift, err := versionutils.CompareVersions(workerGroupVersion, ">", controlPlaneVersion)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, controlPlaneVersion, err.Error()))
	}
	if uplift {
		allErrs = append(allErrs, field.Forbidden(fldPath, "worker group kubernetes version must not be higher than control plane version"))
	}

	// Forbid Kubernetes version upgrade which skips a minor version
	workerVersion, err := semver.NewVersion(workerGroupVersion)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, workerGroupVersion, err.Error()))
	}

	var (
		k8sGreaterEqual128, _ = versionutils.CheckVersionMeetsConstraint(controlPlaneVersion, ">= 1.28")
		minorSkewVersion      = workerVersion.IncMinor().IncMinor().IncMinor()
		maxSkew               = "two"
	)

	if k8sGreaterEqual128 {
		minorSkewVersion = workerVersion.IncMinor().IncMinor().IncMinor().IncMinor()
		maxSkew = "three"
	}

	versionSkewViolation, err := versionutils.CompareVersions(controlPlaneVersion, ">=", minorSkewVersion.String())
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, controlPlaneVersion, err.Error()))
	}
	if versionSkewViolation {
		allErrs = append(allErrs, field.Forbidden(fldPath, "worker group kubernetes version must be at most "+maxSkew+" minor versions behind control plane version"))
	}

	return allErrs
}

func validateDNS(dns *core.DNS, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if dns == nil {
		return allErrs
	}

	if dns.Domain != nil {
		allErrs = append(allErrs, ValidateDNS1123Subdomain(*dns.Domain, fldPath.Child("domain"))...)
	}

	primaryDNSProvider := helper.FindPrimaryDNSProvider(dns.Providers)
	if primaryDNSProvider != nil && primaryDNSProvider.Type != nil {
		if *primaryDNSProvider.Type != core.DNSUnmanaged && dns.Domain == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("domain"), fmt.Sprintf("domain must be set when primary provider type is not set to %q", core.DNSUnmanaged)))
		}
	}

	var (
		names        = sets.New[string]()
		primaryFound bool
	)
	for i, provider := range dns.Providers {
		idxPath := fldPath.Child("providers").Index(i)

		if provider.SecretName != nil && provider.Type != nil {
			providerName := gardenerutils.GenerateDNSProviderName(*provider.SecretName, *provider.Type)
			if names.Has(providerName) {
				allErrs = append(allErrs, field.Invalid(idxPath, providerName, "combination of .secretName and .type must be unique across dns providers"))
				continue
			}
			for _, err := range validation.IsDNS1123Subdomain(providerName) {
				allErrs = append(allErrs, field.Invalid(idxPath, providerName, fmt.Sprintf("combination of .secretName and .type is invalid: %q", err)))
			}
			names.Insert(providerName)
		}

		if provider.Primary != nil && *provider.Primary {
			if primaryFound {
				allErrs = append(allErrs, field.Forbidden(idxPath.Child("primary"), "multiple primary DNS providers are not supported"))
				continue
			}
			primaryFound = true
		}

		if providerType := provider.Type; providerType != nil {
			if *providerType == core.DNSUnmanaged && provider.SecretName != nil {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("secretName"), provider.SecretName, fmt.Sprintf("secretName must not be set when type is %q", core.DNSUnmanaged)))
				continue
			}
		}

		if provider.SecretName != nil && provider.Type == nil {
			allErrs = append(allErrs, field.Required(idxPath.Child("type"), "type must be set when secretName is set"))
		}
	}

	return allErrs
}

func validateKubernetes(kubernetes core.Kubernetes, networking *core.Networking, workerless bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(kubernetes.Version) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("version"), "kubernetes version must not be empty"))
		return allErrs
	}

	if ptr.Deref(kubernetes.EnableStaticTokenKubeconfig, false) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("enableStaticTokenKubeconfig"), *kubernetes.EnableStaticTokenKubeconfig, "setting this field to true is not supported"))
	}

	allErrs = append(allErrs, validateETCD(kubernetes.ETCD, fldPath.Child("etcd"))...)
	allErrs = append(allErrs, ValidateKubeAPIServer(kubernetes.KubeAPIServer, kubernetes.Version, workerless, gardenerutils.DefaultResourcesForEncryption(), fldPath.Child("kubeAPIServer"))...)
	allErrs = append(allErrs, ValidateKubeControllerManager(kubernetes.KubeControllerManager, networking, kubernetes.Version, workerless, fldPath.Child("kubeControllerManager"))...)

	if workerless {
		allErrs = append(allErrs, validateKubernetesForWorkerlessShoot(kubernetes, fldPath)...)
	} else {
		allErrs = append(allErrs, validateKubeScheduler(kubernetes.KubeScheduler, kubernetes.Version, fldPath.Child("kubeScheduler"))...)
		allErrs = append(allErrs, validateKubeProxy(kubernetes.KubeProxy, kubernetes.Version, fldPath.Child("kubeProxy"))...)

		if kubernetes.Kubelet != nil {
			allErrs = append(allErrs, ValidateKubeletConfig(*kubernetes.Kubelet, kubernetes.Version, fldPath.Child("kubelet"))...)
		}

		if clusterAutoscaler := kubernetes.ClusterAutoscaler; clusterAutoscaler != nil {
			allErrs = append(allErrs, ValidateClusterAutoscaler(*clusterAutoscaler, kubernetes.Version, fldPath.Child("clusterAutoscaler"))...)
		}

		if verticalPodAutoscaler := kubernetes.VerticalPodAutoscaler; verticalPodAutoscaler != nil {
			allErrs = append(allErrs, ValidateVerticalPodAutoscaler(*verticalPodAutoscaler, fldPath.Child("verticalPodAutoscaler"))...)
		}
	}

	return allErrs
}

func validateETCD(etcd *core.ETCD, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if etcd != nil {
		if etcd.Main != nil {
			allErrs = append(allErrs, ValidateControlPlaneAutoscaling(etcd.Main.Autoscaling, corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("300M")}, fldPath.Child("main", "autoscaling"))...)
		}

		if etcd.Events != nil {
			allErrs = append(allErrs, ValidateControlPlaneAutoscaling(etcd.Events.Autoscaling, corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("60M")}, fldPath.Child("events", "autoscaling"))...)
		}
	}

	return allErrs
}

func validateKubernetesForWorkerlessShoot(kubernetes core.Kubernetes, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if kubernetes.KubeScheduler != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("kubeScheduler"), workerlessErrorMsg))
	}

	if kubernetes.KubeProxy != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("kubeProxy"), workerlessErrorMsg))
	}

	if kubernetes.Kubelet != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("kubelet"), workerlessErrorMsg))
	}

	if kubernetes.ClusterAutoscaler != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("clusterAutoScaler"), workerlessErrorMsg))
	}

	if kubernetes.VerticalPodAutoscaler != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("verticalPodAutoScaler"), workerlessErrorMsg))
	}

	return allErrs
}

func fieldNilOrEmptyString(field *string) bool {
	return field == nil || len(*field) == 0
}

func validateNetworking(networking *core.Networking, workerless bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if workerless {
		// Nothing to be validated here, exit
		if networking == nil {
			return allErrs
		}

		if networking.Type != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("type"), workerlessErrorMsg))
		}
		if networking.ProviderConfig != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("providerConfig"), workerlessErrorMsg))
		}
		if networking.Pods != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("pods"), workerlessErrorMsg))
		}
		if networking.Nodes != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("nodes"), workerlessErrorMsg))
		}
	} else {
		if networking == nil {
			allErrs = append(allErrs, field.Required(fldPath, "networking should not be nil for a Shoot with workers"))
			return allErrs
		}

		if len(ptr.Deref(networking.Type, "")) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("type"), "networking type must be provided"))
		}
	}

	if errs := ValidateIPFamilies(networking.IPFamilies, fldPath.Child("ipFamilies")); len(errs) > 0 {
		// further validation doesn't make any sense, because we don't know which IP family to check for in the CIDR fields
		return append(allErrs, errs...)
	}

	primaryIPFamily := helper.DeterminePrimaryIPFamily(networking.IPFamilies)

	if networking.Nodes != nil {
		path := fldPath.Child("nodes")
		cidr := cidrvalidation.NewCIDR(*networking.Nodes, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		// For dualstack, primaryIPFamily might not match configured CIDRs
		if len(networking.IPFamilies) < 2 {
			allErrs = append(allErrs, cidr.ValidateIPFamily(string(primaryIPFamily))...)
		}
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	if networking.Pods != nil {
		path := fldPath.Child("pods")
		cidr := cidrvalidation.NewCIDR(*networking.Pods, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		// For dualstack, primaryIPFamily might not match configured CIDRs
		if len(networking.IPFamilies) < 2 {
			allErrs = append(allErrs, cidr.ValidateIPFamily(string(primaryIPFamily))...)
		}
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	if networking.Services != nil {
		path := fldPath.Child("services")
		cidr := cidrvalidation.NewCIDR(*networking.Services, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		// For dualstack, primaryIPFamily might not match configured CIDRs
		if len(networking.IPFamilies) < 2 {
			allErrs = append(allErrs, cidr.ValidateIPFamily(string(primaryIPFamily))...)
		}
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	return allErrs
}

func validateNetworkingStatus(networking *core.NetworkingStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if networking == nil {
		return allErrs
	}

	for i, n := range networking.Nodes {
		path := fldPath.Child("nodes").Index(i)
		cidr := cidrvalidation.NewCIDR(n, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	for i, p := range networking.Pods {
		path := fldPath.Child("pods").Index(i)
		cidr := cidrvalidation.NewCIDR(p, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	for i, s := range networking.Services {
		path := fldPath.Child("services").Index(i)
		cidr := cidrvalidation.NewCIDR(s, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	for i, e := range networking.EgressCIDRs {
		path := fldPath.Child("egressCIDRs").Index(i)
		cidr := cidrvalidation.NewCIDR(e, path)

		allErrs = append(allErrs, cidr.ValidateParse()...)
		allErrs = append(allErrs, cidrvalidation.ValidateCIDRIsCanonical(path, cidr.GetCIDR())...)
	}

	return allErrs
}

// ValidateWatchCacheSizes validates the given WatchCacheSizes fields.
func ValidateWatchCacheSizes(sizes *core.WatchCacheSizes, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if sizes != nil {
		if defaultSize := sizes.Default; defaultSize != nil {
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*defaultSize), fldPath.Child("default"))...)
		}

		for idx, resourceWatchCacheSize := range sizes.Resources {
			idxPath := fldPath.Child("resources").Index(idx)
			if len(resourceWatchCacheSize.Resource) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("resource"), "must not be empty"))
			}
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(resourceWatchCacheSize.CacheSize), idxPath.Child("size"))...)
		}
	}
	return allErrs
}

// ValidateAPIServerLogging validates the given KubeAPIServer Logging fields.
func ValidateAPIServerLogging(loggingConfig *core.APIServerLogging, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if loggingConfig != nil {
		if verbosity := loggingConfig.Verbosity; verbosity != nil {
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*verbosity), fldPath.Child("verbosity"))...)
		}
		if httpAccessVerbosity := loggingConfig.HTTPAccessVerbosity; httpAccessVerbosity != nil {
			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*httpAccessVerbosity), fldPath.Child("httpAccessVerbosity"))...)
		}
	}
	return allErrs
}

// ValidateAPIServerRequests validates the given KubeAPIServer request fields.
func ValidateAPIServerRequests(requests *core.APIServerRequests, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if requests != nil {
		const maxNonMutatingRequestsInflight = 800
		if v := requests.MaxNonMutatingInflight; v != nil {
			path := fldPath.Child("maxNonMutatingInflight")

			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*v), path)...)
			if *v > maxNonMutatingRequestsInflight {
				allErrs = append(allErrs, field.Invalid(path, *v, fmt.Sprintf("cannot set higher than %d", maxNonMutatingRequestsInflight)))
			}
		}

		const maxMutatingRequestsInflight = 400
		if v := requests.MaxMutatingInflight; v != nil {
			path := fldPath.Child("maxMutatingInflight")

			allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*v), path)...)
			if *v > maxMutatingRequestsInflight {
				allErrs = append(allErrs, field.Invalid(path, *v, fmt.Sprintf("cannot set higher than %d", maxMutatingRequestsInflight)))
			}
		}
	}

	return allErrs
}

func validateEncryptionConfig(encryptionConfig *core.EncryptionConfig, defaultEncryptedResources sets.Set[string], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if encryptionConfig == nil {
		return allErrs
	}

	seenResources := sets.New[string]()
	for i, resource := range encryptionConfig.Resources {
		idxPath := fldPath.Child("encryptionConfig", "resources").Index(i)
		// core resources can be mentioned with empty group (eg: secrets.)
		if seenResources.Has(resource) || seenResources.Has(strings.TrimSuffix(resource, ".")) {
			allErrs = append(allErrs, field.Duplicate(idxPath, resource))
		}

		// core resources can be mentioned with empty group (eg: secrets.)
		if defaultEncryptedResources.Has(strings.TrimSuffix(resource, ".")) {
			allErrs = append(allErrs, field.Forbidden(idxPath, fmt.Sprintf("%q are always encrypted", resource)))
		}

		if strings.HasPrefix(resource, "*") {
			allErrs = append(allErrs, field.Invalid(idxPath, resource, "wildcards are not supported"))
		}

		seenResources.Insert(resource)
	}

	return allErrs
}

// ValidateClusterAutoscaler validates the given ClusterAutoscaler fields.
func ValidateClusterAutoscaler(autoScaler core.ClusterAutoscaler, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if threshold := autoScaler.ScaleDownUtilizationThreshold; threshold != nil {
		if *threshold < 0.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUtilizationThreshold"), *threshold, "can not be negative"))
		}
		if *threshold > 1.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUtilizationThreshold"), *threshold, "can not be greater than 1.0"))
		}
	}
	if maxNodeProvisionTime := autoScaler.MaxNodeProvisionTime; maxNodeProvisionTime != nil && maxNodeProvisionTime.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxNodeProvisionTime"), *maxNodeProvisionTime, "can not be negative"))
	}
	if maxGracefulTerminationSeconds := autoScaler.MaxGracefulTerminationSeconds; maxGracefulTerminationSeconds != nil && *maxGracefulTerminationSeconds < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxGracefulTerminationSeconds"), *maxGracefulTerminationSeconds, "can not be negative"))
	}

	if expander := autoScaler.Expander; expander != nil {
		expanderArray := strings.Split(string(*expander), ",")
		for _, exp := range expanderArray {
			if !availableClusterAutoscalerExpanderModes.Has(exp) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("expander"), *expander, sets.List(availableClusterAutoscalerExpanderModes)))
			}
		}
	}

	if startupTaints := autoScaler.StartupTaints; startupTaints != nil {
		allErrs = append(allErrs, validateClusterAutoscalerTaints(startupTaints, "StartupTaints", version, fldPath.Child("startupTaints"))...)
	}

	if statusTaints := autoScaler.StatusTaints; statusTaints != nil {
		allErrs = append(allErrs, validateClusterAutoscalerTaints(statusTaints, "StatusTaints", version, fldPath.Child("statusTaints"))...)
	}

	if ignoreTaints := autoScaler.IgnoreTaints; ignoreTaints != nil {
		allErrs = append(allErrs, validateClusterAutoscalerTaints(ignoreTaints, "IgnoreTaints", version, fldPath.Child("ignoreTaints"))...)
	}

	if newPodScaleUpDelay := autoScaler.NewPodScaleUpDelay; newPodScaleUpDelay != nil && newPodScaleUpDelay.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("newPodScaleUpDelay"), *newPodScaleUpDelay, "can not be negative"))
	}

	if maxEmptyBulkDelete := autoScaler.MaxEmptyBulkDelete; maxEmptyBulkDelete != nil && *maxEmptyBulkDelete < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxEmptyBulkDelete"), *maxEmptyBulkDelete, "can not be negative"))
	}

	return allErrs
}

// ValidateCloudProfileReference validates the given CloudProfileReference fields.
func ValidateCloudProfileReference(cloudProfileReference *core.CloudProfileReference, cloudProfileName *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// TODO(LucaBernstein): For backwards-compatibility, also to test shoots still specifying only cloudProfileName
	//  to be removed after cloudProfileName is deprecated
	if cloudProfileReference == nil && cloudProfileName != nil {
		cloudProfileReference = &core.CloudProfileReference{
			Kind: "CloudProfile",
			Name: *cloudProfileName,
		}
	}

	// Ensure that cloudProfileReference is provided and contains a cloud profile reference of a valid kind, else fail early.
	// Due to the field synchronization in the shoot strategy, it is safe to only check the cloudProfileReference.
	if cloudProfileReference == nil || len(cloudProfileReference.Name) == 0 {
		return append(allErrs, field.Required(fldPath.Child("name"), "must specify a cloud profile"))
	}
	cloudProfileReferenceKind := sets.New(v1beta1constants.CloudProfileReferenceKindCloudProfile, v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile)
	if !cloudProfileReferenceKind.Has(cloudProfileReference.Kind) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), cloudProfileReference.Kind, sets.List(cloudProfileReferenceKind)))
	}
	return allErrs
}

// ValidateVerticalPodAutoscaler validates the given VerticalPodAutoscaler fields.
func ValidateVerticalPodAutoscaler(autoScaler core.VerticalPodAutoscaler, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if autoScaler.EvictAfterOOMThreshold != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(autoScaler.EvictAfterOOMThreshold, fldPath.Child("evictAfterOOMThreshold"))...)
	}
	if autoScaler.UpdaterInterval != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(autoScaler.UpdaterInterval, fldPath.Child("updaterInterval"))...)
	}
	if autoScaler.RecommenderInterval != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(autoScaler.RecommenderInterval, fldPath.Child("recommenderInterval"))...)
	}
	if percentile := autoScaler.TargetCPUPercentile; percentile != nil {
		allErrs = append(allErrs, validatePercentile(*percentile, fldPath.Child("targetCPUPercentile"))...)
	}
	if percentile := autoScaler.RecommendationLowerBoundCPUPercentile; percentile != nil {
		allErrs = append(allErrs, validatePercentile(*percentile, fldPath.Child("recommendationLowerBoundCPUPercentile"))...)
	}
	if percentile := autoScaler.RecommendationUpperBoundCPUPercentile; percentile != nil {
		allErrs = append(allErrs, validatePercentile(*percentile, fldPath.Child("recommendationUpperBoundCPUPercentile"))...)
	}
	if percentile := autoScaler.TargetMemoryPercentile; percentile != nil {
		allErrs = append(allErrs, validatePercentile(*percentile, fldPath.Child("targetMemoryPercentile"))...)
	}
	if percentile := autoScaler.RecommendationLowerBoundMemoryPercentile; percentile != nil {
		allErrs = append(allErrs, validatePercentile(*percentile, fldPath.Child("recommendationLowerBoundMemoryPercentile"))...)
	}
	if percentile := autoScaler.RecommendationUpperBoundMemoryPercentile; percentile != nil {
		allErrs = append(allErrs, validatePercentile(*percentile, fldPath.Child("recommendationUpperBoundMemoryPercentile"))...)
	}
	if autoScaler.CPUHistogramDecayHalfLife != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(autoScaler.CPUHistogramDecayHalfLife, fldPath.Child("cpuHistogramDecayHalfLife"))...)
	}
	if autoScaler.MemoryHistogramDecayHalfLife != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(autoScaler.MemoryHistogramDecayHalfLife, fldPath.Child("memoryHistogramDecayHalfLife"))...)
	}
	if autoScaler.MemoryAggregationInterval != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(autoScaler.MemoryAggregationInterval, fldPath.Child("memoryAggregationInterval"))...)
	}
	if count := autoScaler.MemoryAggregationIntervalCount; count != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(*count, fldPath.Child("memoryAggregationIntervalCount"))...)
	}

	return allErrs
}

func validatePercentile(percentile float64, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if percentile < 0.0 || percentile > 1.0 {
		allErrs = append(allErrs, field.Invalid(fldPath, percentile, "percentile value must be in the range [0, 1]"))
	}

	return allErrs
}

func validateHibernationUpdate(new, old *core.Shoot) field.ErrorList {
	var (
		allErrs                     = field.ErrorList{}
		fldPath                     = field.NewPath("spec", "hibernation", "enabled")
		hibernationEnabledInOld     = old.Spec.Hibernation != nil && ptr.Deref(old.Spec.Hibernation.Enabled, false)
		hibernationEnabledInNew     = new.Spec.Hibernation != nil && ptr.Deref(new.Spec.Hibernation.Enabled, false)
		encryptedResourcesInOldSpec = sets.Set[string]{}
		encryptedResourcesInNewSpec = sets.Set[string]{}
	)

	if old.Spec.Kubernetes.KubeAPIServer != nil && old.Spec.Kubernetes.KubeAPIServer.EncryptionConfig != nil {
		encryptedResourcesInOldSpec.Insert(old.Spec.Kubernetes.KubeAPIServer.EncryptionConfig.Resources...)
	}

	if new.Spec.Kubernetes.KubeAPIServer != nil && new.Spec.Kubernetes.KubeAPIServer.EncryptionConfig != nil {
		encryptedResourcesInNewSpec.Insert(new.Spec.Kubernetes.KubeAPIServer.EncryptionConfig.Resources...)
	}

	if !hibernationEnabledInOld && hibernationEnabledInNew {
		if new.Status.Credentials != nil && new.Status.Credentials.Rotation != nil && new.Status.Credentials.Rotation.ETCDEncryptionKey != nil {
			if etcdEncryptionKeyRotation := new.Status.Credentials.Rotation.ETCDEncryptionKey; etcdEncryptionKeyRotation.Phase == core.RotationPreparing || etcdEncryptionKeyRotation.Phase == core.RotationCompleting {
				allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("shoot cannot be hibernated when .status.credentials.rotation.etcdEncryptionKey.phase is %q", string(etcdEncryptionKeyRotation.Phase))))
			}
		}
		if new.Status.Credentials != nil && new.Status.Credentials.Rotation != nil && new.Status.Credentials.Rotation.ServiceAccountKey != nil {
			if serviceAccountKeyRotation := new.Status.Credentials.Rotation.ServiceAccountKey; sets.New(core.RotationPreparing, core.RotationPreparingWithoutWorkersRollout, core.RotationCompleting).Has(serviceAccountKeyRotation.Phase) {
				allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("shoot cannot be hibernated when .status.credentials.rotation.serviceAccountKey.phase is %q", string(serviceAccountKeyRotation.Phase))))
			}
		}

		if !encryptedResourcesInOldSpec.Equal(sets.New(old.Status.EncryptedResources...)) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "shoot cannot be hibernated when spec.kubernetes.kubeAPIServer.encryptionConfig.resources and status.encryptedResources are not equal"))
		}
	}

	return allErrs
}

// ValidateKubeAPIServer validates KubeAPIServerConfig.
func ValidateKubeAPIServer(kubeAPIServer *core.KubeAPIServerConfig, version string, workerless bool, defaultEncryptedResources sets.Set[string], fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if kubeAPIServer == nil {
		return allErrs
	}

	// TODO(AleksandarSavchev): Remove this check as soon as v1.32 is the least supported Kubernetes version in Gardener.
	k8sGreaterEqual132, _ := versionutils.CheckVersionMeetsConstraint(version, ">= 1.32")
	if oidc := kubeAPIServer.OIDCConfig; k8sGreaterEqual132 && oidc != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("oidcConfig"), *oidc, "for Kubernetes versions >= 1.32, oidcConfig field is no longer supported"))
	} else if oidc != nil {
		oidcPath := fldPath.Child("oidcConfig")

		if fieldNilOrEmptyString(oidc.ClientID) {
			if oidc.ClientID != nil {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("clientID"), oidc.ClientID, "clientID cannot be empty when key is provided"))
			}
			if !fieldNilOrEmptyString(oidc.IssuerURL) {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("clientID"), oidc.ClientID, "clientID must be set when issuerURL is provided"))
			}
		}

		if fieldNilOrEmptyString(oidc.IssuerURL) {
			if oidc.IssuerURL != nil {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("issuerURL"), oidc.IssuerURL, "issuerURL cannot be empty when key is provided"))
			}
			if !fieldNilOrEmptyString(oidc.ClientID) {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("issuerURL"), oidc.IssuerURL, "issuerURL must be set when clientID is provided"))
			}
		} else {
			allErrs = append(allErrs, ValidateOIDCIssuerURL(*oidc.IssuerURL, oidcPath.Child("issuerURL"))...)
		}

		if oidc.CABundle != nil {
			if _, err := utils.DecodeCertificate([]byte(*oidc.CABundle)); err != nil {
				allErrs = append(allErrs, field.Invalid(oidcPath.Child("caBundle"), *oidc.CABundle, "caBundle is not a valid PEM-encoded certificate"))
			}
		}
		// TODO(AleksandarSavchev): Remove this check as soon as v1.31 is the least supported Kubernetes version in Gardener.
		k8sGreaterEqual131, _ := versionutils.CheckVersionMeetsConstraint(version, ">= 1.31")
		if oidc.ClientAuthentication != nil && k8sGreaterEqual131 {
			allErrs = append(allErrs, field.Invalid(oidcPath.Child("clientAuthentication"), *oidc.ClientAuthentication, "for Kubernetes versions >= 1.31, clientAuthentication field is no longer supported"))
		}
		if oidc.GroupsClaim != nil && len(*oidc.GroupsClaim) == 0 {
			allErrs = append(allErrs, field.Invalid(oidcPath.Child("groupsClaim"), *oidc.GroupsClaim, "groupsClaim cannot be empty when key is provided"))
		}
		if oidc.GroupsPrefix != nil && len(*oidc.GroupsPrefix) == 0 {
			allErrs = append(allErrs, field.Invalid(oidcPath.Child("groupsPrefix"), *oidc.GroupsPrefix, "groupsPrefix cannot be empty when key is provided"))
		}
		for i, alg := range oidc.SigningAlgs {
			if !availableOIDCSigningAlgs.Has(alg) {
				allErrs = append(allErrs, field.NotSupported(oidcPath.Child("signingAlgs").Index(i), alg, sets.List(availableOIDCSigningAlgs)))
			}
		}
		if oidc.UsernameClaim != nil && len(*oidc.UsernameClaim) == 0 {
			allErrs = append(allErrs, field.Invalid(oidcPath.Child("usernameClaim"), *oidc.UsernameClaim, "usernameClaim cannot be empty when key is provided"))
		}
		if oidc.UsernamePrefix != nil && len(*oidc.UsernamePrefix) == 0 {
			allErrs = append(allErrs, field.Invalid(oidcPath.Child("usernamePrefix"), *oidc.UsernamePrefix, "usernamePrefix cannot be empty when key is provided"))
		}
	}

	allErrs = append(allErrs, admissionpluginsvalidation.ValidateAdmissionPlugins(kubeAPIServer.AdmissionPlugins, version, fldPath.Child("admissionPlugins"))...)
	allErrs = append(allErrs, apigroupsvalidation.ValidateAPIGroupVersions(kubeAPIServer.RuntimeConfig, version, workerless, fldPath.Child("runtimeConfig"))...)

	if auditConfig := kubeAPIServer.AuditConfig; auditConfig != nil {
		auditPath := fldPath.Child("auditConfig")
		if auditPolicy := auditConfig.AuditPolicy; auditPolicy != nil && auditConfig.AuditPolicy.ConfigMapRef != nil {
			allErrs = append(allErrs, ValidateAuditPolicyConfigMapReference(auditPolicy.ConfigMapRef, auditPath.Child("auditPolicy", "configMapRef"))...)
		}
	}

	k8sLess130, _ := versionutils.CheckVersionMeetsConstraint(version, "< 1.30")
	if structuredAuthentication := kubeAPIServer.StructuredAuthentication; structuredAuthentication != nil {
		structAuthPath := fldPath.Child("structuredAuthentication")
		if k8sLess130 {
			allErrs = append(allErrs, field.Forbidden(structAuthPath, "is available for Kubernetes versions >= v1.30"))
		}
		if value, ok := kubeAPIServer.FeatureGates["StructuredAuthenticationConfiguration"]; ok && !value {
			allErrs = append(allErrs, field.Forbidden(structAuthPath, "requires feature gate StructuredAuthenticationConfiguration to be enabled"))
		}
		if kubeAPIServer.OIDCConfig != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("oidcConfig"), "is incompatible with structuredAuthentication"))
		}
		if len(structuredAuthentication.ConfigMapName) == 0 {
			allErrs = append(allErrs, field.Forbidden(structAuthPath.Child("configMapName"), "must provide a name"))
		}
	}

	if structuredAuthorization := kubeAPIServer.StructuredAuthorization; structuredAuthorization != nil {
		structAuthPath := fldPath.Child("structuredAuthorization")
		if k8sLess130 {
			allErrs = append(allErrs, field.Forbidden(structAuthPath, "is available for Kubernetes versions >= v1.30"))
		}
		if value, ok := kubeAPIServer.FeatureGates["StructuredAuthorizationConfiguration"]; ok && !value {
			allErrs = append(allErrs, field.Forbidden(structAuthPath, "requires feature gate StructuredAuthorizationConfiguration to be enabled"))
		}
		if len(structuredAuthorization.ConfigMapName) == 0 {
			allErrs = append(allErrs, field.Forbidden(structAuthPath.Child("configMapName"), "must provide a name"))
		}
		if structuredAuthorization.ConfigMapName != "" && len(structuredAuthorization.Kubeconfigs) == 0 {
			allErrs = append(allErrs, field.Required(structAuthPath.Child("kubeconfigs"), "must provide kubeconfig secret references if an authorization config is configured"))
		}
	}

	allErrs = append(allErrs, ValidateWatchCacheSizes(kubeAPIServer.WatchCacheSizes, fldPath.Child("watchCacheSizes"))...)

	allErrs = append(allErrs, ValidateAPIServerLogging(kubeAPIServer.Logging, fldPath.Child("logging"))...)

	if defaultNotReadyTolerationSeconds := kubeAPIServer.DefaultNotReadyTolerationSeconds; defaultNotReadyTolerationSeconds != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(*defaultNotReadyTolerationSeconds, fldPath.Child("defaultNotReadyTolerationSeconds"))...)
	}
	if defaultUnreachableTolerationSeconds := kubeAPIServer.DefaultUnreachableTolerationSeconds; defaultUnreachableTolerationSeconds != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(*defaultUnreachableTolerationSeconds, fldPath.Child("defaultUnreachableTolerationSeconds"))...)
	}

	allErrs = append(allErrs, validateEncryptionConfig(kubeAPIServer.EncryptionConfig, defaultEncryptedResources, fldPath)...)

	allErrs = append(allErrs, ValidateAPIServerRequests(kubeAPIServer.Requests, fldPath.Child("requests"))...)

	if kubeAPIServer.ServiceAccountConfig != nil {
		if kubeAPIServer.ServiceAccountConfig.MaxTokenExpiration != nil {
			if kubeAPIServer.ServiceAccountConfig.MaxTokenExpiration.Duration < 0 {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("serviceAccountConfig", "maxTokenExpiration"), *kubeAPIServer.ServiceAccountConfig.MaxTokenExpiration, "can not be negative"))
			}

			if duration := kubeAPIServer.ServiceAccountConfig.MaxTokenExpiration.Duration; duration > 0 && duration < 720*time.Hour {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("serviceAccountConfig", "maxTokenExpiration"), "must be at least 720h (30d)"))
			}

			if duration := kubeAPIServer.ServiceAccountConfig.MaxTokenExpiration.Duration; duration > 2160*time.Hour {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("serviceAccountConfig", "maxTokenExpiration"), "must be at most 2160h (90d)"))
			}
		}
		if len(kubeAPIServer.ServiceAccountConfig.AcceptedIssuers) > 0 {
			issuers := sets.New[string]()
			if kubeAPIServer.ServiceAccountConfig.Issuer != nil {
				issuers.Insert(*kubeAPIServer.ServiceAccountConfig.Issuer)
			}
			for i, acceptedIssuer := range kubeAPIServer.ServiceAccountConfig.AcceptedIssuers {
				if issuers.Has(acceptedIssuer) {
					path := fldPath.Child("serviceAccountConfig", "acceptedIssuers").Index(i)
					if issuer := kubeAPIServer.ServiceAccountConfig.Issuer; issuer != nil && *issuer == acceptedIssuer {
						allErrs = append(allErrs, field.Invalid(path, acceptedIssuer, fmt.Sprintf("acceptedIssuers cannot contains the issuer field value: %s", acceptedIssuer)))
					} else {
						allErrs = append(allErrs, field.Duplicate(path, acceptedIssuer))
					}
				} else {
					issuers.Insert(acceptedIssuer)
				}
			}
		}
	}

	if kubeAPIServer.EventTTL != nil {
		if kubeAPIServer.EventTTL.Duration < 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("eventTTL"), *kubeAPIServer.EventTTL, "can not be negative"))
		}
		if kubeAPIServer.EventTTL.Duration > time.Hour*24*7 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("eventTTL"), *kubeAPIServer.EventTTL, "can not be longer than 7d"))
		}
	}

	allErrs = append(allErrs, ValidateControlPlaneAutoscaling(
		kubeAPIServer.Autoscaling,
		corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("20m"),
			corev1.ResourceMemory: resource.MustParse("200M"),
		},
		fldPath.Child("autoscaling"))...,
	)

	allErrs = append(allErrs, featuresvalidation.ValidateFeatureGates(kubeAPIServer.FeatureGates, version, fldPath.Child("featureGates"))...)

	return allErrs
}

// ValidateOIDCIssuerURL validates if the given issuerURL follow the expected format.
func ValidateOIDCIssuerURL(issuerURL string, issuerFldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	issuer, err := url.Parse(issuerURL)
	if err != nil || (issuer != nil && len(issuer.Host) == 0) {
		allErrs = append(allErrs, field.Invalid(issuerFldPath, issuerURL, "must be a valid URL and have https scheme"))
	}
	if issuer != nil && issuer.Fragment != "" {
		allErrs = append(allErrs, field.Invalid(issuerFldPath, issuerURL, "must not contain a fragment"))
	}
	if issuer != nil && issuer.User != nil {
		allErrs = append(allErrs, field.Invalid(issuerFldPath, issuerURL, "must not contain a username or password"))
	}
	if issuer != nil && len(issuer.RawQuery) > 0 {
		allErrs = append(allErrs, field.Invalid(issuerFldPath, issuerURL, "must not contain a query"))
	}
	if issuer != nil && issuer.Scheme != "https" {
		allErrs = append(allErrs, field.Invalid(issuerFldPath, issuerURL, "must have https scheme"))
	}

	return allErrs
}

// ValidateKubeControllerManager validates KubeControllerManagerConfig.
func ValidateKubeControllerManager(kcm *core.KubeControllerManagerConfig, networking *core.Networking, version string, workerless bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if kcm == nil {
		return nil
	}

	if !workerless {
		if maskSize := kcm.NodeCIDRMaskSize; maskSize != nil && networking != nil {
			if core.IsIPv4SingleStack(networking.IPFamilies) {
				if *maskSize < 16 || *maskSize > 28 {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("nodeCIDRMaskSize"), *maskSize, "nodeCIDRMaskSize must be between 16 and 28"))
				}
			}
		}

		if podEvictionTimeout := kcm.PodEvictionTimeout; podEvictionTimeout != nil && podEvictionTimeout.Duration <= 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("podEvictionTimeout"), podEvictionTimeout.Duration, "podEvictionTimeout must be larger than 0"))
		}

		if nodeMonitorGracePeriod := kcm.NodeMonitorGracePeriod; nodeMonitorGracePeriod != nil && nodeMonitorGracePeriod.Duration <= 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("nodeMonitorGracePeriod"), nodeMonitorGracePeriod.Duration, "nodeMonitorGracePeriod must be larger than 0"))
		}

		if hpa := kcm.HorizontalPodAutoscalerConfig; hpa != nil {
			hpaPath := fldPath.Child("horizontalPodAutoscaler")

			if hpa.SyncPeriod != nil && hpa.SyncPeriod.Duration < 1*time.Second {
				allErrs = append(allErrs, field.Invalid(hpaPath.Child("syncPeriod"), *hpa.SyncPeriod, "syncPeriod must not be less than a second"))
			}
			if hpa.Tolerance != nil && *hpa.Tolerance <= 0 {
				allErrs = append(allErrs, field.Invalid(hpaPath.Child("tolerance"), *hpa.Tolerance, "tolerance of must be greater than 0"))
			}
			if hpa.DownscaleStabilization != nil && hpa.DownscaleStabilization.Duration < 1*time.Second {
				allErrs = append(allErrs, field.Invalid(hpaPath.Child("downscaleStabilization"), *hpa.DownscaleStabilization, "downScale stabilization must not be less than a second"))
			}
			if hpa.InitialReadinessDelay != nil && hpa.InitialReadinessDelay.Duration <= 0 {
				allErrs = append(allErrs, field.Invalid(hpaPath.Child("initialReadinessDelay"), *hpa.InitialReadinessDelay, "initial readiness delay must be greater than 0"))
			}
			if hpa.CPUInitializationPeriod != nil && hpa.CPUInitializationPeriod.Duration < 1*time.Second {
				allErrs = append(allErrs, field.Invalid(hpaPath.Child("cpuInitializationPeriod"), *hpa.CPUInitializationPeriod, "cpu initialization period must not be less than a second"))
			}
		}
	} else {
		if kcm.NodeCIDRMaskSize != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("nodeCIDRMaskSize"), workerlessErrorMsg))
		}
		if kcm.HorizontalPodAutoscalerConfig != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("horizontalPodAutoscaler"), workerlessErrorMsg))
		}
		if kcm.PodEvictionTimeout != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("podEvictionTimeout"), workerlessErrorMsg))
		}
		if kcm.NodeMonitorGracePeriod != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("nodeMonitorGracePeriod"), workerlessErrorMsg))
		}
	}

	allErrs = append(allErrs, featuresvalidation.ValidateFeatureGates(kcm.FeatureGates, version, fldPath.Child("featureGates"))...)

	return allErrs
}

func validateKubeScheduler(ks *core.KubeSchedulerConfig, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if ks != nil {
		profile := ks.Profile
		if profile != nil {
			if !availableSchedulingProfiles.Has(string(*profile)) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child("profile"), *profile, sets.List(availableSchedulingProfiles)))
			}
		}

		allErrs = append(allErrs, featuresvalidation.ValidateFeatureGates(ks.FeatureGates, version, fldPath.Child("featureGates"))...)
	}

	return allErrs
}

func validateKubeProxy(kp *core.KubeProxyConfig, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if kp != nil {
		if kp.Mode == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("mode"), "must be set when .spec.kubernetes.kubeProxy is set"))
		} else if mode := *kp.Mode; !availableProxyModes.Has(string(mode)) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("mode"), mode, sets.List(availableProxyModes)))
		}
		allErrs = append(allErrs, featuresvalidation.ValidateFeatureGates(kp.FeatureGates, version, fldPath.Child("featureGates"))...)
	}
	return allErrs
}

func validateMonitoring(monitoring *core.Monitoring, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if monitoring != nil && monitoring.Alerting != nil {
		allErrs = append(allErrs, validateAlerting(monitoring.Alerting, fldPath.Child("alerting"))...)
	}
	return allErrs
}

func validateAlerting(alerting *core.Alerting, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	emails := make(map[string]struct{})
	for i, email := range alerting.EmailReceivers {
		if !utils.TestEmail(email) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("emailReceivers").Index(i), email, "must provide a valid email"))
		}

		if _, duplicate := emails[email]; duplicate {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("emailReceivers").Index(i), email))
		} else {
			emails[email] = struct{}{}
		}
	}
	return allErrs
}

func validateMaintenance(maintenance *core.Maintenance, fldPath *field.Path, workerless bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if maintenance == nil {
		return allErrs
	}

	if maintenance.AutoUpdate != nil {
		if workerless && maintenance.AutoUpdate.MachineImageVersion != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("autoUpdate", "machineImageVersion"), workerlessErrorMsg))
		}
	}

	if maintenance.TimeWindow != nil {
		maintenanceTimeWindow, err := timewindow.ParseMaintenanceTimeWindow(maintenance.TimeWindow.Begin, maintenance.TimeWindow.End)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("timeWindow", "begin/end"), maintenance.TimeWindow, err.Error()))
		} else {
			duration := maintenanceTimeWindow.Duration()
			if duration > core.MaintenanceTimeWindowDurationMaximum {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("timeWindow"), duration, fmt.Sprintf("time window must not be greater than %s", core.MaintenanceTimeWindowDurationMaximum)))
				return allErrs
			}
			if duration < core.MaintenanceTimeWindowDurationMinimum {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("timeWindow"), duration, fmt.Sprintf("time window must not be smaller than %s", core.MaintenanceTimeWindowDurationMinimum)))
				return allErrs
			}
		}
	}

	return allErrs
}

func validateProvider(provider core.Provider, kubernetes core.Kubernetes, networking *core.Networking, workerless bool, fldPath *field.Path, inTemplate bool) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		maxPod  int32
	)

	if len(provider.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must specify a provider type"))
	}

	if workerless {
		if provider.InfrastructureConfig != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("infrastructureConfig"), workerlessErrorMsg))
		}
		if provider.ControlPlaneConfig != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("controlPlaneConfig"), workerlessErrorMsg))
		}
		if provider.WorkersSettings != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("workersSettings"), workerlessErrorMsg))
		}
	} else {
		if kubernetes.Kubelet != nil && kubernetes.Kubelet.MaxPods != nil {
			maxPod = *kubernetes.Kubelet.MaxPods
		}

		for i, worker := range provider.Workers {
			allErrs = append(allErrs, ValidateWorker(worker, kubernetes, fldPath.Child("workers").Index(i), inTemplate)...)

			if worker.Kubernetes != nil && worker.Kubernetes.Kubelet != nil && worker.Kubernetes.Kubelet.MaxPods != nil && *worker.Kubernetes.Kubelet.MaxPods > maxPod {
				maxPod = *worker.Kubernetes.Kubelet.MaxPods
			}
		}

		allErrs = append(allErrs, ValidateWorkers(provider.Workers, fldPath.Child("workers"))...)
		allErrs = append(allErrs, ValidateSystemComponentWorkers(provider.Workers, fldPath.Child("workers"))...)
	}

	if kubernetes.KubeControllerManager != nil && kubernetes.KubeControllerManager.NodeCIDRMaskSize != nil && networking != nil {
		if maxPod == 0 {
			// default maxPod setting on kubelet
			maxPod = 110
		}
		allErrs = append(allErrs, ValidateNodeCIDRMaskWithMaxPod(maxPod, *kubernetes.KubeControllerManager.NodeCIDRMaskSize, *networking)...)
	}

	return allErrs
}

const (
	// maxWorkerNameLength is a constant for the maximum length for worker name.
	maxWorkerNameLength = 15

	// maxVolumeNameLength is a constant for the maximum length for data volume name.
	maxVolumeNameLength = 15
)

// volumeSizeRegex is used for volume size validation.
var volumeSizeRegex = regexp.MustCompile(`^(\d)+Gi$`)

// ValidateWorker validates the worker object.
func ValidateWorker(worker core.Worker, kubernetes core.Kubernetes, fldPath *field.Path, inTemplate bool) field.ErrorList {
	kubernetesVersion := kubernetes.Version
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, validateDNS1123Label(worker.Name, fldPath.Child("name"))...)
	if len(worker.Name) > maxWorkerNameLength {
		allErrs = append(allErrs, field.TooLong(fldPath.Child("name"), worker.Name, maxWorkerNameLength))
	}
	if len(worker.Machine.Type) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("machine", "type"), "must specify a machine type"))
	}
	if worker.Machine.Image != nil {
		if len(worker.Machine.Image.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("machine", "image", "name"), "must specify a machine image name"))
		}
		if !inTemplate && len(worker.Machine.Image.Version) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("machine", "image", "version"), "must specify a machine image version"))
		}
	}
	if worker.Minimum < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("minimum"), worker.Minimum, "minimum value must not be negative"))
	}
	if worker.Maximum < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maximum"), worker.Maximum, "maximum value must not be negative"))
	}
	if worker.Maximum < worker.Minimum {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("maximum"), "maximum value must not be less than minimum value"))
	}

	allErrs = append(allErrs, ValidatePositiveIntOrPercent(worker.MaxSurge, fldPath.Child("maxSurge"))...)
	allErrs = append(allErrs, ValidatePositiveIntOrPercent(worker.MaxUnavailable, fldPath.Child("maxUnavailable"))...)
	allErrs = append(allErrs, IsNotMoreThan100Percent(worker.MaxUnavailable, fldPath.Child("maxUnavailable"))...)

	if getIntOrPercentValue(ptr.Deref(worker.MaxSurge, intstr.IntOrString{})) != 0 && ptr.Deref(worker.UpdateStrategy, "") == core.ManualInPlaceUpdate {
		// MaxSurge must be 0 when update strategy is ManualInPlaceUpdate.
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxSurge"), worker.MaxSurge, "must be 0 when `updateStrategy` is `ManualInPlaceUpdate`"))
	}

	if (worker.MaxUnavailable == nil || getIntOrPercentValue(*worker.MaxUnavailable) == 0) && (worker.MaxSurge != nil && getIntOrPercentValue(*worker.MaxSurge) == 0) {
		// Both MaxSurge and MaxUnavailable cannot be zero.
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxUnavailable"), worker.MaxUnavailable, "may not be 0 when `maxSurge` is 0"))
	}

	allErrs = append(allErrs, metav1validation.ValidateLabels(worker.Labels, fldPath.Child("labels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(worker.Annotations, fldPath.Child("annotations"))...)
	if len(worker.Taints) > 0 {
		allErrs = append(allErrs, validateTaints(worker.Taints, fldPath.Child("taints"))...)
	}
	if worker.Kubernetes != nil {
		if worker.Kubernetes.Version != nil {
			workerGroupKubernetesVersion := *worker.Kubernetes.Version
			allErrs = append(allErrs, validateWorkerGroupAndControlPlaneKubernetesVersion(kubernetesVersion, workerGroupKubernetesVersion, fldPath.Child("kubernetes", "version"))...)
			kubernetesVersion = workerGroupKubernetesVersion
		}

		if worker.Kubernetes.Kubelet != nil {
			allErrs = append(allErrs, ValidateKubeletConfig(*worker.Kubernetes.Kubelet, kubernetesVersion, fldPath.Child("kubernetes", "kubelet"))...)
		} else if kubernetes.Kubelet != nil {
			allErrs = append(allErrs, ValidateKubeletConfig(*kubernetes.Kubelet, kubernetesVersion, fldPath.Child("kubernetes", "kubelet"))...)
		}
	}

	if worker.CABundle != nil {
		if _, err := utils.DecodeCertificate([]byte(*worker.CABundle)); err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("caBundle"), *(worker.CABundle), "caBundle is not a valid PEM-encoded certificate"))
		}
	}

	if worker.Volume != nil {
		if !volumeSizeRegex.MatchString(worker.Volume.VolumeSize) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("volume", "size"), worker.Volume.VolumeSize, fmt.Sprintf("volume size must match the regex %s", volumeSizeRegex)))
		}
	}

	if worker.DataVolumes != nil {
		volumeNames := make(map[string]int)
		if len(worker.DataVolumes) > 0 && worker.Volume == nil {
			allErrs = append(allErrs, field.Required(fldPath.Child("volume"), "a worker volume must be defined if data volumes are defined"))
		}
		for idx, volume := range worker.DataVolumes {
			idxPath := fldPath.Child("dataVolumes").Index(idx)
			if len(volume.Name) == 0 {
				allErrs = append(allErrs, field.Required(idxPath.Child("name"), "must specify a name"))
			} else {
				allErrs = append(allErrs, validateDNS1123Label(volume.Name, idxPath.Child("name"))...)
			}
			if len(volume.Name) > maxVolumeNameLength {
				allErrs = append(allErrs, field.TooLong(idxPath.Child("name"), volume.Name, maxVolumeNameLength))
			}
			if _, keyExist := volumeNames[volume.Name]; keyExist {
				volumeNames[volume.Name]++
				allErrs = append(allErrs, field.Duplicate(idxPath.Child("name"), volume.Name))
			} else {
				volumeNames[volume.Name] = 1
			}
			if !volumeSizeRegex.MatchString(volume.VolumeSize) {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("size"), volume.VolumeSize, fmt.Sprintf("data volume size must match the regex %s", volumeSizeRegex)))
			}
		}
	}

	if worker.KubeletDataVolumeName != nil {
		found := false
		for _, volume := range worker.DataVolumes {
			if volume.Name == *worker.KubeletDataVolumeName {
				found = true
			}
		}
		if !found {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("kubeletDataVolumeName"), worker.KubeletDataVolumeName, fmt.Sprintf("KubeletDataVolumeName refers to unrecognized data volume %s", *worker.KubeletDataVolumeName)))
		}
	}

	if worker.CRI != nil {
		allErrs = append(allErrs, ValidateCRI(worker.CRI, fldPath.Child("cri"))...)
	}

	if worker.Machine.Architecture != nil {
		allErrs = append(allErrs, ValidateArchitecture(worker.Machine.Architecture, fldPath.Child("machine", "architecture"))...)
	}

	if worker.ClusterAutoscaler != nil {
		allErrs = append(allErrs, ValidateClusterAutoscalerOptions(worker.ClusterAutoscaler, fldPath.Child("autoscaler"))...)
	}

	if worker.MachineControllerManagerSettings != nil && ptr.Deref(worker.MachineControllerManagerSettings.DisableHealthTimeout, false) && !helper.IsUpdateStrategyInPlace(worker.UpdateStrategy) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("machineControllerManagerSettings", "disableHealthTimeout"), "can only be set to true when the update strategy is `AutoInPlaceUpdate` or `ManualInPlaceUpdate`"))
	}

	if worker.UpdateStrategy != nil {
		if !availableUpdateStrategies.Has(*worker.UpdateStrategy) {
			allErrs = append(allErrs, field.NotSupported(fldPath.Child("updateStrategy"), *worker.UpdateStrategy, sets.List(availableUpdateStrategies)))
		}

		if !features.DefaultFeatureGate.Enabled(features.InPlaceNodeUpdates) && helper.IsUpdateStrategyInPlace(worker.UpdateStrategy) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("updateStrategy"), *worker.UpdateStrategy, "can not configure `AutoInPlaceUpdate` or `ManualInPlaceUpdate` update strategies when the `InPlaceNodeUpdates` feature gate is disabled."))
		}
	}

	return allErrs
}

// ValidateClusterAutoscalerOptions validates the cluster autoscaler options of worker pools.
func ValidateClusterAutoscalerOptions(caOptions *core.ClusterAutoscalerOptions, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if scaleDownUtilThreshold := caOptions.ScaleDownUtilizationThreshold; scaleDownUtilThreshold != nil {
		if *scaleDownUtilThreshold < 0.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUtilizationThreshold"), *scaleDownUtilThreshold, "can not be negative"))
		}
		if *scaleDownUtilThreshold > 1.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUtilizationThreshold"), *scaleDownUtilThreshold, "can not be greater than 1.0"))
		}
	}
	if scaleDownGpuUtilThreshold := caOptions.ScaleDownGpuUtilizationThreshold; scaleDownGpuUtilThreshold != nil {
		if *scaleDownGpuUtilThreshold < 0.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownGpuUtilizationThreshold"), *scaleDownGpuUtilThreshold, "can not be negative"))
		}
		if *scaleDownGpuUtilThreshold > 1.0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownGpuUtilizationThreshold"), *scaleDownGpuUtilThreshold, "can not be greater than 1.0"))
		}
	}
	if scaleDownUnneededTime := caOptions.ScaleDownUnneededTime; scaleDownUnneededTime != nil && scaleDownUnneededTime.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUnneededTime"), *scaleDownUnneededTime, "can not be negative"))
	}
	if scaleDownUnreadyTime := caOptions.ScaleDownUnreadyTime; scaleDownUnreadyTime != nil && scaleDownUnreadyTime.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("scaleDownUnreadyTime"), *scaleDownUnreadyTime, "can not be negative"))
	}
	if maxNodeProvisionTime := caOptions.MaxNodeProvisionTime; maxNodeProvisionTime != nil && maxNodeProvisionTime.Duration < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("maxNodeProvisionTime"), *maxNodeProvisionTime, "can not be negative"))
	}

	return allErrs
}

// PodPIDsLimitMinimum is a constant for the minimum value for the podPIDsLimit field.
const PodPIDsLimitMinimum int64 = 100

// ValidateKubeletConfig validates the KubeletConfig object.
func ValidateKubeletConfig(kubeletConfig core.KubeletConfig, version string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if kubeletConfig.MaxPods != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*kubeletConfig.MaxPods), fldPath.Child("maxPods"))...)
	}
	if value := kubeletConfig.PodPIDsLimit; value != nil {
		if *value < PodPIDsLimitMinimum {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("podPIDsLimit"), *value, fmt.Sprintf("podPIDsLimit value must be at least %d", PodPIDsLimitMinimum)))
		}
	}
	if kubeletConfig.EvictionPressureTransitionPeriod != nil {
		allErrs = append(allErrs, ValidatePositiveDuration(kubeletConfig.EvictionPressureTransitionPeriod, fldPath.Child("evictionPressureTransitionPeriod"))...)
	}
	if kubeletConfig.EvictionMaxPodGracePeriod != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*kubeletConfig.EvictionMaxPodGracePeriod), fldPath.Child("evictionMaxPodGracePeriod"))...)
	}
	if kubeletConfig.EvictionHard != nil {
		allErrs = append(allErrs, validateKubeletConfigEviction(kubeletConfig.EvictionHard, fldPath.Child("evictionHard"))...)
	}
	if kubeletConfig.EvictionSoft != nil {
		allErrs = append(allErrs, validateKubeletConfigEviction(kubeletConfig.EvictionSoft, fldPath.Child("evictionSoft"))...)
	}
	if kubeletConfig.EvictionMinimumReclaim != nil {
		allErrs = append(allErrs, validateKubeletConfigEvictionMinimumReclaim(kubeletConfig.EvictionMinimumReclaim, fldPath.Child("evictionMinimumReclaim"))...)
	}
	if kubeletConfig.EvictionSoftGracePeriod != nil {
		allErrs = append(allErrs, validateKubeletConfigEvictionSoftGracePeriod(kubeletConfig.EvictionSoftGracePeriod, fldPath.Child("evictionSoftGracePeriod"))...)
	}
	if kubeletConfig.KubeReserved != nil {
		allErrs = append(allErrs, validateKubeletConfigReserved(kubeletConfig.KubeReserved, fldPath.Child("kubeReserved"))...)
	}
	if kubeletConfig.SystemReserved != nil {
		allErrs = append(allErrs, validateKubeletConfigReserved(kubeletConfig.SystemReserved, fldPath.Child("systemReserved"))...)

		if k8sGreaterEqual131, _ := versionutils.CheckVersionMeetsConstraint(version, ">= 1.31"); k8sGreaterEqual131 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("systemReserved"), kubeletConfig.SystemReserved, "systemReserved is no longer supported by Gardener starting from Kubernetes 1.31"))
		}
	}
	if v := kubeletConfig.ImageGCHighThresholdPercent; v != nil && (*v < 0 || *v > 100) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("imageGCHighThresholdPercent"), *v, "value must be in [0,100]"))
	}
	if v := kubeletConfig.ImageGCLowThresholdPercent; v != nil && (*v < 0 || *v > 100) {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("imageGCLowThresholdPercent"), *v, "value must be in [0,100]"))
	}
	if kubeletConfig.ImageGCHighThresholdPercent != nil && kubeletConfig.ImageGCLowThresholdPercent != nil && *kubeletConfig.ImageGCLowThresholdPercent >= *kubeletConfig.ImageGCHighThresholdPercent {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("imageGCLowThresholdPercent"), "imageGCLowThresholdPercent must be less than imageGCHighThresholdPercent"))
	}
	allErrs = append(allErrs, featuresvalidation.ValidateFeatureGates(kubeletConfig.FeatureGates, version, fldPath.Child("featureGates"))...)
	if v := kubeletConfig.RegistryPullQPS; v != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*v), fldPath.Child("registryPullQPS"))...)
	}
	if v := kubeletConfig.RegistryBurst; v != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*v), fldPath.Child("registryBurst"))...)
	}
	if v := kubeletConfig.ContainerLogMaxFiles; v != nil {
		if *v < 2 {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("containerLogMaxFiles"), *v, "value must be >= 2."))
		}
	}
	if v := kubeletConfig.StreamingConnectionIdleTimeout; v != nil {
		if v.Duration < time.Second*30 || time.Hour*4 < v.Duration {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("streamingConnectionIdleTimeout"), *v, "value must be between 30s and 4h"))
		}
	}

	if v := kubeletConfig.MemorySwap; v != nil {
		path := fldPath.Child("memorySwap")

		if ptr.Deref(kubeletConfig.FailSwapOn, false) {
			allErrs = append(allErrs, field.Forbidden(path, "configuring swap behaviour is not available when the kubelet is configured with 'FailSwapOn=true'"))
		}

		if v.SwapBehavior != nil {
			if featureGateEnabled, ok := kubeletConfig.FeatureGates["NodeSwap"]; !ok || (!featureGateEnabled) {
				allErrs = append(allErrs, field.Forbidden(path, "configuring swap behaviour is not available when kubelet's 'NodeSwap' feature gate is not set"))
			}

			supportedSwapBehaviors := []core.SwapBehavior{core.LimitedSwap, core.UnlimitedSwap}
			k8sGreaterEqual130, _ := versionutils.CheckVersionMeetsConstraint(version, ">= 1.30")
			if k8sGreaterEqual130 {
				supportedSwapBehaviors = []core.SwapBehavior{core.NoSwap, core.LimitedSwap}
			}

			if !slices.Contains(supportedSwapBehaviors, *v.SwapBehavior) {
				allErrs = append(allErrs, field.NotSupported(path.Child("swapBehavior"), *v.SwapBehavior, supportedSwapBehaviors))
			}
		}
	}

	return allErrs
}

func validateKubeletConfigEviction(eviction *core.KubeletConfigEviction, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.MemoryAvailable, fldPath, "memoryAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSAvailable, fldPath, "imagefsAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSInodesFree, fldPath, "imagefsInodesFree")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.NodeFSAvailable, fldPath, "nodefsAvailable")...)
	allErrs = append(allErrs, ValidateResourceQuantityOrPercent(eviction.ImageFSInodesFree, fldPath, "imagefsInodesFree")...)
	return allErrs
}

func validateKubeletConfigEvictionMinimumReclaim(eviction *core.KubeletConfigEvictionMinimumReclaim, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if eviction.MemoryAvailable != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("memoryAvailable", *eviction.MemoryAvailable, fldPath.Child("memoryAvailable"))...)
	}
	if eviction.ImageFSAvailable != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("imagefsAvailable", *eviction.ImageFSAvailable, fldPath.Child("imagefsAvailable"))...)
	}
	if eviction.ImageFSInodesFree != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("imagefsInodesFree", *eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	}
	if eviction.NodeFSAvailable != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("nodefsAvailable", *eviction.NodeFSAvailable, fldPath.Child("nodefsAvailable"))...)
	}
	if eviction.ImageFSInodesFree != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("imagefsInodesFree", *eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	}
	return allErrs
}

func validateKubeletConfigEvictionSoftGracePeriod(eviction *core.KubeletConfigEvictionSoftGracePeriod, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.MemoryAvailable, fldPath.Child("memoryAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSAvailable, fldPath.Child("imagefsAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.NodeFSAvailable, fldPath.Child("nodefsAvailable"))...)
	allErrs = append(allErrs, ValidatePositiveDuration(eviction.ImageFSInodesFree, fldPath.Child("imagefsInodesFree"))...)
	return allErrs
}

func validateKubeletConfigReserved(reserved *core.KubeletConfigReserved, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if reserved.CPU != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("cpu", *reserved.CPU, fldPath.Child("cpu"))...)
	}
	if reserved.Memory != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("memory", *reserved.Memory, fldPath.Child("memory"))...)
	}
	if reserved.EphemeralStorage != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("ephemeralStorage", *reserved.EphemeralStorage, fldPath.Child("ephemeralStorage"))...)
	}
	if reserved.PID != nil {
		allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue("pid", *reserved.PID, fldPath.Child("pid"))...)
	}
	return allErrs
}

var reservedTaintKeys = sets.New(v1beta1constants.TaintNodeCriticalComponentsNotReady)

func validateClusterAutoscalerTaints(taints []string, option string, version string, fldPath *field.Path) field.ErrorList {
	var optionVersionRanges = map[string]*featuresvalidation.FeatureGateVersionRange{
		"IgnoreTaints":  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.14", RemovedInVersion: "1.32"}},
		"StartupTaints": {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
		"StatusTaints":  {VersionRange: versionutils.VersionRange{AddedInVersion: "1.29"}},
	}

	allErrs := field.ErrorList{}

	supported, err := optionVersionRanges[option].Contains(version)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(option), option, err.Error()))
	} else if !supported {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child(option), "not supported in Kubernetes version "+version))
	}

	taintKeySet := make(map[string]struct{})

	for i, taint := range taints {
		idxPath := fldPath.Index(i)

		// validate the taint key
		allErrs = append(allErrs, metav1validation.ValidateLabelName(taint, idxPath)...)

		// deny reserved taint keys
		if reservedTaintKeys.Has(taint) {
			allErrs = append(allErrs, field.Forbidden(idxPath, "taint key is reserved by gardener"))
		}

		// validate if taint key is duplicate
		if _, ok := taintKeySet[taint]; ok {
			allErrs = append(allErrs, field.Duplicate(idxPath, taint))
			continue
		}
		taintKeySet[taint] = struct{}{}
	}
	return allErrs
}

// https://github.com/kubernetes/kubernetes/blob/ee9079f8ec39914ff8975b5390749771b9303ea4/pkg/apis/core/validation/validation.go#L4057-L4089
func validateTaints(taints []corev1.Taint, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	uniqueTaints := map[corev1.TaintEffect]sets.Set[string]{}

	for i, taint := range taints {
		idxPath := fldPath.Index(i)
		// validate the taint key
		allErrs = append(allErrs, metav1validation.ValidateLabelName(taint.Key, idxPath.Child("key"))...)

		// deny reserved taint keys
		if reservedTaintKeys.Has(taint.Key) {
			allErrs = append(allErrs, field.Forbidden(idxPath.Child("key"), "taint key is reserved by gardener"))
		}

		// validate the taint value
		if errs := validation.IsValidLabelValue(taint.Value); len(errs) != 0 {
			allErrs = append(allErrs, field.Invalid(idxPath.Child("value"), taint.Value, strings.Join(errs, ";")))
		}
		// validate the taint effect
		allErrs = append(allErrs, validateTaintEffect(&taint.Effect, false, idxPath.Child("effect"))...)

		// validate if taint is unique by <key, effect>
		if len(uniqueTaints[taint.Effect]) > 0 && uniqueTaints[taint.Effect].Has(taint.Key) {
			duplicatedError := field.Duplicate(idxPath, taint)
			duplicatedError.Detail = "taints must be unique by key and effect pair"
			allErrs = append(allErrs, duplicatedError)

			continue
		}

		// add taint to existingTaints for uniqueness check
		if len(uniqueTaints[taint.Effect]) == 0 {
			uniqueTaints[taint.Effect] = sets.Set[string]{}
		}
		uniqueTaints[taint.Effect].Insert(taint.Key)
	}
	return allErrs
}

// https://github.com/kubernetes/kubernetes/blob/ee9079f8ec39914ff8975b5390749771b9303ea4/pkg/apis/core/validation/validation.go#L2774-L2795
func validateTaintEffect(effect *corev1.TaintEffect, allowEmpty bool, fldPath *field.Path) field.ErrorList {
	if !allowEmpty && len(*effect) == 0 {
		return field.ErrorList{field.Required(fldPath, "")}
	}

	allErrors := field.ErrorList{}

	switch *effect {
	case corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute:
	default:
		validValues := []string{
			string(corev1.TaintEffectNoSchedule),
			string(corev1.TaintEffectPreferNoSchedule),
			string(corev1.TaintEffectNoExecute),
		}
		allErrors = append(allErrors, field.NotSupported(fldPath, *effect, validValues))
	}
	return allErrors
}

// ValidateWorkers validates worker objects.
func ValidateWorkers(workers []core.Worker, fldPath *field.Path) field.ErrorList {
	var (
		allErrs     = field.ErrorList{}
		workerNames = sets.New[string]()
	)

	for i, worker := range workers {
		if workerNames.Has(worker.Name) {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("name"), worker.Name))
		}
		workerNames.Insert(worker.Name)

		if worker.ControlPlane != nil {
			allErrs = append(allErrs, field.Forbidden(fldPath.Index(i).Child("controlPlane"), "setting controlPlane is not allowed in worker configuration currently"))
		}
	}

	return allErrs
}

// ValidateSystemComponentWorkers validates workers specified to run system components.
func ValidateSystemComponentWorkers(workers []core.Worker, fldPath *field.Path) field.ErrorList {
	var (
		allErrs                                   = field.ErrorList{}
		atLeastOnePoolWithAllowedSystemComponents = false

		workerPoolsWithSufficientWorkers   = make(map[string]struct{})
		workerPoolsWithInsufficientWorkers = make(map[string]int)
	)

	for i, worker := range workers {
		// check if system component worker pool is configured
		if !helper.SystemComponentsAllowed(&worker) {
			continue
		}

		if worker.Maximum == 0 {
			continue
		}
		atLeastOnePoolWithAllowedSystemComponents = true

		// Check if the maximum worker count is greater than or equal to the number of specified zones.
		// It ensures that the cluster has at least one worker per zone in order to schedule required system components with TopologySpreadConstraints.
		// This check is done per distinct worker pool concerning their zone setup,
		// e.g. 'worker[x].zones: {1,2,3}' is the same as 'worker[y].zones: {3,2,1}', so the constraint is only considered once for both worker groups.
		zonesSet := sets.New(worker.Zones...)

		var (
			hasSufficientWorkers = false
			workerPoolKey        = strings.Join(sets.List(zonesSet), "--")
		)

		if int(worker.Maximum) >= len(worker.Zones) {
			hasSufficientWorkers = true
		}

		if hasSufficientWorkers {
			workerPoolsWithSufficientWorkers[workerPoolKey] = struct{}{}

			delete(workerPoolsWithInsufficientWorkers, workerPoolKey)
		} else {
			if _, b := workerPoolsWithSufficientWorkers[workerPoolKey]; !b {
				workerPoolsWithInsufficientWorkers[workerPoolKey] = i
			}
		}
	}

	for _, i := range workerPoolsWithInsufficientWorkers {
		allErrs = append(allErrs, field.Forbidden(fldPath.Index(i).Child("maximum"), "maximum node count should be greater than or equal to the number of zones specified for this pool"))
	}

	if !atLeastOnePoolWithAllowedSystemComponents {
		allErrs = append(allErrs, field.Forbidden(fldPath, "at least one active (workers[i].maximum > 0) worker pool with systemComponents.allow=true needed"))
	}

	return allErrs
}

// ValidateHibernation validates a Hibernation object.
func ValidateHibernation(annotations map[string]string, hibernation *core.Hibernation, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if hibernation == nil {
		return allErrs
	}

	if maintenanceOp := annotations[v1beta1constants.GardenerMaintenanceOperation]; forbiddenShootOperationsWhenHibernated.Has(maintenanceOp) && ptr.Deref(hibernation.Enabled, false) {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("enabled"), fmt.Sprintf("shoot cannot be hibernated when %s=%s annotation is set", v1beta1constants.GardenerMaintenanceOperation, maintenanceOp)))
	}

	allErrs = append(allErrs, ValidateHibernationSchedules(hibernation.Schedules, fldPath.Child("schedules"))...)

	return allErrs
}

// ValidateHibernationSchedules validates a list of hibernation schedules.
func ValidateHibernationSchedules(schedules []core.HibernationSchedule, fldPath *field.Path) field.ErrorList {
	var (
		allErrs = field.ErrorList{}
		seen    = sets.New[string]()
	)

	for i, schedule := range schedules {
		allErrs = append(allErrs, ValidateHibernationSchedule(seen, &schedule, fldPath.Index(i))...)
	}

	return allErrs
}

// ValidateHibernationCronSpec validates a cron specification of a hibernation schedule.
func ValidateHibernationCronSpec(seenSpecs sets.Set[string], spec string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	_, err := cron.ParseStandard(spec)

	switch {
	case err != nil:
		allErrs = append(allErrs, field.Invalid(fldPath, spec, fmt.Sprintf("not a valid cron spec: %v", err)))
	case seenSpecs.Has(spec):
		allErrs = append(allErrs, field.Duplicate(fldPath, spec))
	default:
		seenSpecs.Insert(spec)
	}

	return allErrs
}

// ValidateHibernationScheduleLocation validates that the location of a HibernationSchedule is correct.
func ValidateHibernationScheduleLocation(location string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if _, err := time.LoadLocation(location); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, location, fmt.Sprintf("not a valid location: %v", err)))
	}

	return allErrs
}

// ValidateHibernationSchedule validates the correctness of a HibernationSchedule.
// It checks whether the set start and end time are valid cron specs.
func ValidateHibernationSchedule(seenSpecs sets.Set[string], schedule *core.HibernationSchedule, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if schedule.Start == nil && schedule.End == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("start/end"), "either start or end has to be provided"))
	}
	if schedule.Start != nil {
		allErrs = append(allErrs, ValidateHibernationCronSpec(seenSpecs, *schedule.Start, fldPath.Child("start"))...)
	}
	if schedule.End != nil {
		allErrs = append(allErrs, ValidateHibernationCronSpec(seenSpecs, *schedule.End, fldPath.Child("end"))...)
	}
	if schedule.Location != nil {
		allErrs = append(allErrs, ValidateHibernationScheduleLocation(*schedule.Location, fldPath.Child("location"))...)
	}

	return allErrs
}

// ValidatePositiveDuration validates that a duration is positive.
func ValidatePositiveDuration(duration *metav1.Duration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if duration == nil {
		return allErrs
	}
	if duration.Seconds() < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, duration.Duration.String(), "must be non-negative"))
	}
	return allErrs
}

// ValidateResourceQuantityOrPercent checks if a value can be parsed to either a resource.quantity, a positive int or percent.
func ValidateResourceQuantityOrPercent(valuePtr *string, fldPath *field.Path, key string) field.ErrorList {
	allErrs := field.ErrorList{}

	if valuePtr == nil {
		return allErrs
	}
	value := *valuePtr
	// check for resource quantity
	if quantity, err := resource.ParseQuantity(value); err == nil {
		if len(kubernetescorevalidation.ValidateResourceQuantityValue(key, quantity, fldPath)) == 0 {
			return allErrs
		}
	}

	if validation.IsValidPercent(value) != nil {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(key), value, "field must be either a valid resource quantity (e.g 200Mi) or a percentage (e.g '5%')"))
		return allErrs
	}

	percentValue, _ := strconv.Atoi(value[:len(value)-1])
	if percentValue > 100 || percentValue < 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(key), value, "must not be greater than 100% and not smaller than 0%"))
	}
	return allErrs
}

// ValidatePositiveIntOrPercent validates a int or string object and ensures it is positive.
func ValidatePositiveIntOrPercent(intOrPercent *intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if intOrPercent == nil {
		return allErrs
	}

	switch intOrPercent.Type {
	case intstr.String:
		if validation.IsValidPercent(intOrPercent.StrVal) != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, intOrPercent, "must be an integer or percentage (e.g '5%')"))
		}
	case intstr.Int:
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(intOrPercent.IntValue()), fldPath)...)
	}

	return allErrs
}

// IsNotMoreThan100Percent validates an int or string object and ensures it is not more than 100%.
func IsNotMoreThan100Percent(intOrStringValue *intstr.IntOrString, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if intOrStringValue == nil {
		return allErrs
	}

	value, isPercent := getPercentValue(*intOrStringValue)
	if !isPercent || value <= 100 {
		return nil
	}
	allErrs = append(allErrs, field.Invalid(fldPath, intOrStringValue, "must not be greater than 100%"))

	return allErrs
}

// ValidateCRI validates container runtime interface name and its container runtimes
func ValidateCRI(CRI *core.CRI, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if !availableWorkerCRINames.Has(string(CRI.Name)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("name"), string(CRI.Name), sets.List(availableWorkerCRINames)))
	}

	if CRI.ContainerRuntimes != nil {
		allErrs = append(allErrs, ValidateContainerRuntimes(CRI.ContainerRuntimes, fldPath.Child("containerruntimes"))...)
	}

	return allErrs
}

// ValidateContainerRuntimes validates the given container runtimes
func ValidateContainerRuntimes(containerRuntime []core.ContainerRuntime, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	crSet := make(map[string]bool)

	for i, cr := range containerRuntime {
		if len(cr.Type) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Index(i).Child("type"), "must specify a container runtime type"))
		}
		if crSet[cr.Type] {
			allErrs = append(allErrs, field.Duplicate(fldPath.Index(i).Child("type"), fmt.Sprintf("must specify different type, %s already exist", cr.Type)))
		}
		crSet[cr.Type] = true
	}

	return allErrs
}

// ValidateArchitecture validates the CPU architecture of the machines in this worker pool.
func ValidateArchitecture(arch *string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if !slices.Contains(v1beta1constants.ValidArchitectures, *arch) {
		allErrs = append(allErrs, field.NotSupported(fldPath, *arch, v1beta1constants.ValidArchitectures))
	}

	return allErrs
}

// ValidateSystemComponents validates the given system components.
func ValidateSystemComponents(systemComponents *core.SystemComponents, fldPath *field.Path, workerless bool) field.ErrorList {
	allErrs := field.ErrorList{}

	if systemComponents == nil {
		return allErrs
	} else if workerless {
		allErrs = append(allErrs, field.Forbidden(fldPath, workerlessErrorMsg))
		return allErrs
	}

	allErrs = append(allErrs, validateCoreDNS(systemComponents.CoreDNS, fldPath.Child("coreDNS"))...)

	return allErrs
}

// validateCoreDNS validates the given Core DNS settings.
func validateCoreDNS(coreDNS *core.CoreDNS, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if coreDNS == nil {
		return allErrs
	}

	if coreDNS.Autoscaling != nil && !availableCoreDNSAutoscalingModes.Has(string(coreDNS.Autoscaling.Mode)) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("autoscaling").Child("mode"), coreDNS.Autoscaling.Mode, sets.List(availableCoreDNSAutoscalingModes)))
	}
	if coreDNS.Rewriting != nil {
		allErrs = append(allErrs, ValidateCoreDNSRewritingCommonSuffixes(coreDNS.Rewriting.CommonSuffixes, fldPath.Child("rewriting"))...)
	}

	return allErrs
}

// ValidateFinalizersOnCreation validates the finalizers of a Shoot object.
func ValidateFinalizersOnCreation(finalizers []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, finalizer := range finalizers {
		if ForbiddenShootFinalizersOnCreation.Has(finalizer) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Index(i), fmt.Sprintf("finalizer %q cannot be added on creation", finalizer)))
		}
	}

	return allErrs
}

// ValidateCoreDNSRewritingCommonSuffixes validates the given common suffixes used for DNS rewriting.
func ValidateCoreDNSRewritingCommonSuffixes(commonSuffixes []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(commonSuffixes) == 0 {
		return allErrs
	}

	suffixes := map[string]struct{}{}
	for i, s := range commonSuffixes {
		if strings.Count(s, ".") < 1 || (s[0] == '.' && strings.Count(s, ".") < 2) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("commonSuffixes").Index(i), s, "must contain at least one non-leading dot ('.')"))
		}
		s = strings.TrimPrefix(s, ".")
		if _, found := suffixes[s]; found {
			allErrs = append(allErrs, field.Duplicate(fldPath.Child("commonSuffixes").Index(i), s))
		} else {
			suffixes[s] = struct{}{}
		}
	}

	return allErrs
}

func validateShootOperation(operation, maintenanceOperation string, shoot *core.Shoot, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if operation == "" && maintenanceOperation == "" {
		return allErrs
	}

	fldPathOp := fldPath.Key(v1beta1constants.GardenerOperation)
	fldPathMaintOp := fldPath.Key(v1beta1constants.GardenerMaintenanceOperation)

	if operation == maintenanceOperation {
		allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("annotations %s and %s must not be equal", fldPathOp, fldPathMaintOp)))
	}

	// TODO(rfranzke): Remove this block once the CredentialsRotationWithoutWorkersRollout feature gate gets promoted
	//  to GA.
	if !features.DefaultFeatureGate.Enabled(features.CredentialsRotationWithoutWorkersRollout) {
		restrictedOperations := sets.New(
			v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
			v1beta1constants.OperationRotateCAStartWithoutWorkersRollout,
			v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout,
		)
		if operation != "" && (restrictedOperations.Has(operation) || strings.HasPrefix(operation, v1beta1constants.OperationRotateRolloutWorkers)) {
			allErrs = append(allErrs, field.Forbidden(fldPathOp, fmt.Sprintf("the %s operation can only be used when the CredentialsRotationWithoutWorkersRollout feature gate is enabled", operation)))
		}
		if maintenanceOperation != "" && (restrictedOperations.Has(maintenanceOperation) || strings.HasPrefix(maintenanceOperation, v1beta1constants.OperationRotateRolloutWorkers)) {
			allErrs = append(allErrs, field.Forbidden(fldPathMaintOp, fmt.Sprintf("the %s operation can only be used when the CredentialsRotationWithoutWorkersRollout feature gate is enabled", maintenanceOperation)))
		}
	}

	if operation != "" {
		if !availableShootOperations.Has(operation) && !strings.HasPrefix(operation, v1beta1constants.OperationRotateRolloutWorkers) {
			allErrs = append(allErrs, field.NotSupported(fldPathOp, operation, sets.List(availableShootOperations)))
		}
		if helper.IsShootInHibernation(shoot) &&
			(forbiddenShootOperationsWhenHibernated.Has(operation) || strings.HasPrefix(operation, v1beta1constants.OperationRotateRolloutWorkers)) {
			allErrs = append(allErrs, field.Forbidden(fldPathOp, "operation is not permitted when shoot is hibernated or is waking up"))
		}
		if !apiequality.Semantic.DeepEqual(getResourcesForEncryption(shoot.Spec.Kubernetes.KubeAPIServer), shoot.Status.EncryptedResources) &&
			forbiddenShootOperationsWhenEncryptionChangeIsRollingOut.Has(operation) {
			allErrs = append(allErrs, field.Forbidden(fldPathOp, "operation is not permitted because a previous encryption configuration change is currently being rolled out"))
		}
	}

	if maintenanceOperation != "" {
		if !availableShootMaintenanceOperations.Has(maintenanceOperation) && !strings.HasPrefix(maintenanceOperation, v1beta1constants.OperationRotateRolloutWorkers) {
			allErrs = append(allErrs, field.NotSupported(fldPathMaintOp, maintenanceOperation, sets.List(availableShootMaintenanceOperations)))
		}
		if helper.IsShootInHibernation(shoot) &&
			(forbiddenShootOperationsWhenHibernated.Has(maintenanceOperation) || strings.HasPrefix(maintenanceOperation, v1beta1constants.OperationRotateRolloutWorkers)) {
			allErrs = append(allErrs, field.Forbidden(fldPathMaintOp, "operation is not permitted when shoot is hibernated or is waking up"))
		}
		if !apiequality.Semantic.DeepEqual(getResourcesForEncryption(shoot.Spec.Kubernetes.KubeAPIServer), shoot.Status.EncryptedResources) && forbiddenShootOperationsWhenEncryptionChangeIsRollingOut.Has(maintenanceOperation) {
			allErrs = append(allErrs, field.Forbidden(fldPathMaintOp, "operation is not permitted because a previous encryption configuration change is currently being rolled out"))
		}
	}

	switch maintenanceOperation {
	case v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout:
		if sets.New(v1beta1constants.OperationRotateCAStart, v1beta1constants.OperationRotateCAStartWithoutWorkersRollout, v1beta1constants.OperationRotateServiceAccountKeyStart, v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout, v1beta1constants.OperationRotateETCDEncryptionKeyStart).Has(operation) {
			allErrs = append(allErrs, field.Forbidden(fldPathOp, fmt.Sprintf("operation '%s' is not permitted when maintenance operation is '%s'", operation, maintenanceOperation)))
		}
	case v1beta1constants.OperationRotateCredentialsComplete:
		if sets.New(v1beta1constants.OperationRotateCAComplete, v1beta1constants.OperationRotateServiceAccountKeyComplete, v1beta1constants.OperationRotateETCDEncryptionKeyComplete).Has(operation) {
			allErrs = append(allErrs, field.Forbidden(fldPathOp, fmt.Sprintf("operation '%s' is not permitted when maintenance operation is '%s'", operation, maintenanceOperation)))
		}
	}

	switch operation {
	case v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout:
		if sets.New(v1beta1constants.OperationRotateCAStart, v1beta1constants.OperationRotateCAStartWithoutWorkersRollout, v1beta1constants.OperationRotateServiceAccountKeyStart, v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout, v1beta1constants.OperationRotateETCDEncryptionKeyStart).Has(maintenanceOperation) {
			allErrs = append(allErrs, field.Forbidden(fldPathOp, fmt.Sprintf("operation '%s' is not permitted when maintenance operation is '%s'", operation, maintenanceOperation)))
		}
	case v1beta1constants.OperationRotateCredentialsComplete:
		if sets.New(v1beta1constants.OperationRotateCAComplete, v1beta1constants.OperationRotateServiceAccountKeyComplete, v1beta1constants.OperationRotateETCDEncryptionKeyComplete).Has(maintenanceOperation) {
			allErrs = append(allErrs, field.Forbidden(fldPathOp, fmt.Sprintf("operation '%s' is not permitted when maintenance operation is '%s'", operation, maintenanceOperation)))
		}
	}

	allErrs = append(allErrs, validateShootOperationContext(operation, shoot, fldPathOp)...)
	if shoot.DeletionTimestamp == nil {
		// Only validate maintenance operation context when shoot has no deletion timestamp. If it has such a timestamp,
		// any validation is pointless since there are no maintenance operations for shoots in deletion, so we basically
		// don't care. Without this, we could wrongly prevent metadata changes in case the annotation is still present
		// but the shoot is in deletion.
		allErrs = append(allErrs, validateShootOperationContext(maintenanceOperation, shoot, fldPathMaintOp)...)
	}

	return allErrs
}

func validateShootOperationContext(operation string, shoot *core.Shoot, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	switch operation {
	case v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout:
		if !isShootReadyForRotationStart(shoot.Status.LastOperation) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if shoot was not yet created successfully or is not ready for reconciliation"))
		}
		if phase := helper.GetShootCARotationPhase(shoot.Status.Credentials); len(phase) > 0 && phase != core.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
		if phase := helper.GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials); len(phase) > 0 && phase != core.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.serviceAccountKey.phase is not 'Completed'"))
		}
		if phase := helper.GetShootETCDEncryptionKeyRotationPhase(shoot.Status.Credentials); len(phase) > 0 && phase != core.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start rotation of all credentials if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
		allErrs = append(allErrs, validatePendingWorkerUpdates(shoot, fldPath, "rotation of all credentials")...)

	case v1beta1constants.OperationRotateCredentialsComplete:
		if helper.GetShootCARotationPhase(shoot.Status.Credentials) != core.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}
		if helper.GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials) != core.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.serviceAccountKey.phase is not 'Prepared'"))
		}
		if helper.GetShootETCDEncryptionKeyRotationPhase(shoot.Status.Credentials) != core.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete rotation of all credentials if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateCAStart, v1beta1constants.OperationRotateCAStartWithoutWorkersRollout:
		if !isShootReadyForRotationStart(shoot.Status.LastOperation) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if shoot was not yet created successfully or is not ready for reconciliation"))
		}
		if phase := helper.GetShootCARotationPhase(shoot.Status.Credentials); len(phase) > 0 && phase != core.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Completed'"))
		}
		allErrs = append(allErrs, validatePendingWorkerUpdates(shoot, fldPath, "CA rotation")...)

	case v1beta1constants.OperationRotateCAComplete:
		if helper.GetShootCARotationPhase(shoot.Status.Credentials) != core.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete CA rotation if .status.credentials.rotation.certificateAuthorities.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateServiceAccountKeyStart, v1beta1constants.OperationRotateServiceAccountKeyStartWithoutWorkersRollout:
		if !isShootReadyForRotationStart(shoot.Status.LastOperation) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start service account key rotation if shoot was not yet created successfully or is not ready for reconciliation"))
		}
		if phase := helper.GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials); len(phase) > 0 && phase != core.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start service account key rotation if .status.credentials.rotation.serviceAccountKey.phase is not 'Completed'"))
		}
		allErrs = append(allErrs, validatePendingWorkerUpdates(shoot, fldPath, "service account key rotation")...)

	case v1beta1constants.OperationRotateServiceAccountKeyComplete:
		if helper.GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials) != core.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete service account key rotation if .status.credentials.rotation.serviceAccountKey.phase is not 'Prepared'"))
		}

	case v1beta1constants.OperationRotateETCDEncryptionKeyStart:
		if !isShootReadyForRotationStart(shoot.Status.LastOperation) {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if shoot was not yet created successfully or is not ready for reconciliation"))
		}
		if phase := helper.GetShootETCDEncryptionKeyRotationPhase(shoot.Status.Credentials); len(phase) > 0 && phase != core.RotationCompleted {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot start ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Completed'"))
		}
	case v1beta1constants.OperationRotateETCDEncryptionKeyComplete:
		if helper.GetShootETCDEncryptionKeyRotationPhase(shoot.Status.Credentials) != core.RotationPrepared {
			allErrs = append(allErrs, field.Forbidden(fldPath, "cannot complete ETCD encryption key rotation if .status.credentials.rotation.etcdEncryptionKey.phase is not 'Prepared'"))
		}
	}

	if strings.HasPrefix(operation, v1beta1constants.OperationRotateRolloutWorkers) {
		if caPhase, serviceAccountKeyPhase := helper.GetShootCARotationPhase(shoot.Status.Credentials), helper.GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials); caPhase != core.RotationWaitingForWorkersRollout && serviceAccountKeyPhase != core.RotationWaitingForWorkersRollout {
			allErrs = append(allErrs, field.Forbidden(fldPath, "either .status.credentials.rotation.certificateAuthorities.phase or .status.credentials.rotation.serviceAccountKey.phase must be in 'WaitingForWorkersRollout' in order to trigger workers rollout"))
		}

		poolNames := strings.Split(strings.TrimPrefix(operation, v1beta1constants.OperationRotateRolloutWorkers+"="), ",")
		if len(poolNames) == 0 || sets.New(poolNames...).Has("") {
			allErrs = append(allErrs, field.Required(fldPath, "must provide at least one pool name via "+v1beta1constants.OperationRotateRolloutWorkers+"=<poolName1>[,<poolName2>,...]"))
		}

		names := sets.New[string]()
		for _, poolName := range poolNames {
			if names.Has(poolName) {
				allErrs = append(allErrs, field.Duplicate(fldPath, "pool name "+poolName+" was specified multiple times"))
			}
			names.Insert(poolName)

			if !slices.ContainsFunc(shoot.Spec.Provider.Workers, func(worker core.Worker) bool {
				return worker.Name == poolName
			}) {
				allErrs = append(allErrs, field.Invalid(fldPath, poolName, "worker pool name "+poolName+" does not exist in .spec.provider.workers[]"))
			}
		}
	}

	return allErrs
}

// validatePendingWorkerUpdates checks if there are pending workers rollouts in the Shoot's status and returns an error if any are found.
func validatePendingWorkerUpdates(shoot *core.Shoot, fldPath *field.Path, operation string) field.ErrorList {
	allErrs := field.ErrorList{}

	var forbiddenPendingWorkerUpdatesMessageTemplate = "cannot start %s if status.inPlaceUpdates.pendingWorkerUpdates.%s is not empty"
	if shoot.Status.InPlaceUpdates != nil && shoot.Status.InPlaceUpdates.PendingWorkerUpdates != nil {
		if len(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate) > 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf(forbiddenPendingWorkerUpdatesMessageTemplate, operation, "autoInPlaceUpdate")))
		}
		if len(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate) > 0 {
			allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf(forbiddenPendingWorkerUpdatesMessageTemplate, operation, "manualInPlaceUpdate")))
		}
	}

	return allErrs
}

// ValidateForceDeletion validates the addition of force-deletion annotation on the Shoot.
func ValidateForceDeletion(newShoot, oldShoot *core.Shoot) field.ErrorList {
	var (
		fldPath               = field.NewPath("metadata", "annotations").Key(v1beta1constants.AnnotationConfirmationForceDeletion)
		allErrs               = field.ErrorList{}
		oldNeedsForceDeletion = helper.ShootNeedsForceDeletion(oldShoot)
		newNeedsForceDeletion = helper.ShootNeedsForceDeletion(newShoot)
	)

	if !newNeedsForceDeletion && oldNeedsForceDeletion {
		allErrs = append(allErrs, field.Forbidden(fldPath, "force-deletion annotation cannot be removed once set"))
		return allErrs
	}

	if newNeedsForceDeletion && !oldNeedsForceDeletion {
		if newShoot.DeletionTimestamp == nil {
			allErrs = append(allErrs, field.Forbidden(fldPath, "force-deletion annotation cannot be set when Shoot deletionTimestamp is nil"))
		}

		errorCodePresent := false
		for _, lastError := range newShoot.Status.LastErrors {
			if errorCodesAllowingForceDeletion.HasAny(lastError.Codes...) {
				errorCodePresent = true
				break
			}
		}
		if !errorCodePresent {
			allErrs = append(allErrs, field.Forbidden(fldPath, fmt.Sprintf("force-deletion annotation cannot be set when Shoot status does not contain one of these error codes: %v", sets.List(errorCodesAllowingForceDeletion))))
		}
	}

	return allErrs
}

// ValidateInPlaceUpdates validates the updates of the worker pools with in-place update strategy of a Shoot.
func ValidateInPlaceUpdates(newShoot, oldShoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}

	// Allow the update without validation if the force-update annotation is present.
	if kubernetesutils.HasMetaDataAnnotation(newShoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationForceInPlaceUpdate) {
		return allErrs
	}

	if newShoot.Status.InPlaceUpdates == nil || newShoot.Status.InPlaceUpdates.PendingWorkerUpdates == nil {
		return allErrs
	}

	pendingWorkerNames := sets.New[string]()
	pendingWorkerNames.Insert(newShoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate...)
	pendingWorkerNames.Insert(newShoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate...)

	oldControlPlaneKubernetesVersion, err := semver.NewVersion(oldShoot.Spec.Kubernetes.Version)
	if err != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "kubernetes", "version"), "old control plane kubernetes version is not a valid semver version"))
		return allErrs
	}

	newControlPlaneKubernetesVersion, err := semver.NewVersion(newShoot.Spec.Kubernetes.Version)
	if err != nil {
		allErrs = append(allErrs, field.Forbidden(field.NewPath("spec", "kubernetes", "version"), "new control plane kubernetes version is not a valid semver version"))
		return allErrs
	}

	for i, worker := range newShoot.Spec.Provider.Workers {
		if !helper.IsUpdateStrategyInPlace(worker.UpdateStrategy) {
			continue
		}

		if !pendingWorkerNames.Has(worker.Name) {
			continue
		}

		var (
			oldWorker core.Worker
			idxPath   = field.NewPath("spec", "provider", "workers").Index(i)
		)

		oldWorkerIndex := slices.IndexFunc(oldShoot.Spec.Provider.Workers, func(ow core.Worker) bool {
			oldWorker = ow
			return ow.Name == worker.Name
		})

		if oldWorkerIndex == -1 {
			continue
		}

		oldWorkerIndexPath := field.NewPath("spec", "provider", "workers").Index(oldWorkerIndex)

		oldKubernetesVersion, err := helper.CalculateEffectiveKubernetesVersion(oldControlPlaneKubernetesVersion, oldWorker.Kubernetes)
		if err != nil {
			allErrs = append(allErrs, field.Forbidden(oldWorkerIndexPath, fmt.Sprintf("failed to calculate effective kubernetes version for old worker: %v", err)))
			continue
		}
		newKubernetesVersion, err := helper.CalculateEffectiveKubernetesVersion(newControlPlaneKubernetesVersion, worker.Kubernetes)
		if err != nil {
			allErrs = append(allErrs, field.Forbidden(idxPath, fmt.Sprintf("failed to calculate effective kubernetes version for new worker: %v", err)))
			continue
		}

		oldKubeletConfig := helper.CalculateEffectiveKubeletConfiguration(oldShoot.Spec.Kubernetes.Kubelet, oldWorker.Kubernetes)
		newKubeletConfig := helper.CalculateEffectiveKubeletConfiguration(newShoot.Spec.Kubernetes.Kubelet, worker.Kubernetes)

		if !apiequality.Semantic.DeepEqual(oldWorker, worker) || !oldKubernetesVersion.Equal(newKubernetesVersion) || !apiequality.Semantic.DeepEqual(oldKubeletConfig, newKubeletConfig) {
			allErrs = append(allErrs, field.Forbidden(idxPath, fmt.Sprintf("the worker pool %q is currently undergoing an in-place update. No changes are allowed to the worker pool, the Shoot Kubernetes version, or the Shoot kubelet configuration. You can force an update with annotating the Shoot with '%s=%s'", worker.Name, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationForceInPlaceUpdate)))
		}
	}

	return allErrs
}

// ValidateShootHAConfig enforces that both annotation and HA spec are not set together.
func ValidateShootHAConfig(shoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateHAShootControlPlaneConfigurationValue(shoot)...)
	return allErrs
}

// ValidateShootHAConfigUpdate validates the HA shoot control plane configuration.
func ValidateShootHAConfigUpdate(newShoot, oldShoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}
	allErrs = append(allErrs, validateShootHAControlPlaneSpecUpdate(newShoot, oldShoot, field.NewPath("spec.controlPlane"))...)
	return allErrs
}

func validateHAShootControlPlaneConfigurationValue(shoot *core.Shoot) field.ErrorList {
	allErrs := field.ErrorList{}
	if shoot.Spec.ControlPlane != nil && shoot.Spec.ControlPlane.HighAvailability != nil {
		allErrs = append(allErrs, ValidateFailureToleranceTypeValue(shoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type, field.NewPath("spec", "controlPlane", "highAvailability", "failureTolerance", "type"))...)
	}
	return allErrs
}

func validateShootHAControlPlaneSpecUpdate(newShoot, oldShoot *core.Shoot, fldPath *field.Path) field.ErrorList {
	var (
		allErrs          = field.ErrorList{}
		shootIsScheduled = newShoot.Spec.SeedName != nil

		oldVal, newVal core.FailureToleranceType
		oldValExists   bool
	)

	if oldShoot.Spec.ControlPlane != nil && oldShoot.Spec.ControlPlane.HighAvailability != nil {
		oldVal = oldShoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type
		oldValExists = true
	}

	if newShoot.Spec.ControlPlane != nil && newShoot.Spec.ControlPlane.HighAvailability != nil {
		newVal = newShoot.Spec.ControlPlane.HighAvailability.FailureTolerance.Type
		// TODO(@aaronfern): remove this validation of not allowing scale-up to HA while hibernated when https://github.com/gardener/etcd-druid/issues/589 is resolved
		if !oldValExists && helper.IsShootInHibernation(newShoot) {
			allErrs = append(allErrs, field.Forbidden(fldPath.Child("highAvailability", "failureTolerance", "type"), "Shoot is currently hibernated and cannot be scaled up to HA. Please make sure your cluster has woken up before scaling it up to HA"))
		}
	}

	if oldValExists && shootIsScheduled {
		// If the HighAvailability field is already set for the shoot then enforce that it cannot be changed.
		allErrs = append(allErrs, apivalidation.ValidateImmutableField(newVal, oldVal, fldPath.Child("highAvailability", "failureTolerance", "type"))...)
	}

	return allErrs
}

func isShootReadyForRotationStart(lastOperation *core.LastOperation) bool {
	if lastOperation == nil {
		return false
	}
	if lastOperation.Type == core.LastOperationTypeCreate && lastOperation.State == core.LastOperationStateSucceeded {
		return true
	}
	if lastOperation.Type == core.LastOperationTypeRestore && lastOperation.State == core.LastOperationStateSucceeded {
		return true
	}
	return lastOperation.Type == core.LastOperationTypeReconcile
}

func getResourcesForEncryption(apiServerConfig *core.KubeAPIServerConfig) []string {
	resources := sets.New[string]()

	if apiServerConfig != nil && apiServerConfig.EncryptionConfig != nil {
		for _, res := range apiServerConfig.EncryptionConfig.Resources {
			resources.Insert(res)
		}
	}

	return sets.List(resources)
}

// ValidateControlPlaneAutoscaling validates the given auto-scaling configuration.
func ValidateControlPlaneAutoscaling(autoscaling *core.ControlPlaneAutoscaling, minRequired corev1.ResourceList, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if autoscaling != nil {
		if len(autoscaling.MinAllowed) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("minAllowed"), "must provide minAllowed"))
			return allErrs
		}

		allowedResources := sets.New(corev1.ResourceCPU, corev1.ResourceMemory)
		for resource, quantity := range autoscaling.MinAllowed {
			resourcePath := fldPath.Child("minAllowed", resource.String())
			if !allowedResources.Has(resource) {
				allErrs = append(allErrs, field.NotSupported(resourcePath, resource, allowedResources.UnsortedList()))
			}

			if minValue, ok := minRequired[resource]; ok && quantity.Cmp(minValue) < 0 {
				allErrs = append(allErrs, field.Invalid(resourcePath, quantity, fmt.Sprintf("value must be bigger than >= %s", minValue.String())))
			}

			allErrs = append(allErrs, kubernetescorevalidation.ValidateResourceQuantityValue(resource.String(), quantity, resourcePath)...)
		}
	}

	return allErrs
}
