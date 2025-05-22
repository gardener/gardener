// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/timewindow"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// RespectShootSyncPeriodOverwrite checks whether to respect the sync period overwrite of a Shoot or not.
func RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot) bool {
	return respectSyncPeriodOverwrite || shoot.Namespace == v1beta1constants.GardenNamespace
}

// ShouldIgnoreShoot determines whether a Shoot should be ignored or not.
func ShouldIgnoreShoot(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot) bool {
	if !RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot) {
		return false
	}

	value, ok := shoot.Annotations[v1beta1constants.ShootIgnore]
	if !ok {
		return false
	}

	ignore, _ := strconv.ParseBool(value)
	return ignore
}

// IsShootFailedAndUpToDate checks if a Shoot is failed and the observed generation and gardener version are up-to-date.
func IsShootFailedAndUpToDate(shoot *gardencorev1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation

	return lastOperation != nil && lastOperation.State == gardencorev1beta1.LastOperationStateFailed &&
		shoot.Generation == shoot.Status.ObservedGeneration &&
		shoot.Status.Gardener.Version == version.Get().GitVersion
}

// IsNowInEffectiveShootMaintenanceTimeWindow checks if the current time is in the effective
// maintenance time window of the Shoot.
func IsNowInEffectiveShootMaintenanceTimeWindow(shoot *gardencorev1beta1.Shoot, clock clock.Clock) bool {
	return EffectiveShootMaintenanceTimeWindow(shoot).Contains(clock.Now())
}

// LastReconciliationDuringThisTimeWindow returns true if <now> is contained in the given effective maintenance time
// window of the shoot and if the <lastReconciliation> did not happen longer than the longest possible duration of a
// maintenance time window.
func LastReconciliationDuringThisTimeWindow(shoot *gardencorev1beta1.Shoot, clock clock.Clock) bool {
	if shoot.Status.LastOperation == nil {
		return false
	}

	var (
		timeWindow         = EffectiveShootMaintenanceTimeWindow(shoot)
		now                = clock.Now()
		lastReconciliation = shoot.Status.LastOperation.LastUpdateTime.Time
	)

	return timeWindow.Contains(lastReconciliation) && now.UTC().Sub(lastReconciliation.UTC()) <= gardencorev1beta1.MaintenanceTimeWindowDurationMaximum
}

// IsObservedAtLatestGenerationAndSucceeded checks whether the Shoot's generation has changed or if the LastOperation status
// is Succeeded.
func IsObservedAtLatestGenerationAndSucceeded(shoot *gardencorev1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration &&
		(lastOperation != nil && lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded)
}

// SyncPeriodOfShoot determines the sync period of the given shoot.
//
// If no overwrite is allowed, the defaultMinSyncPeriod is returned.
// Otherwise, the overwrite is parsed. If an error occurs or it is smaller than the defaultMinSyncPeriod,
// the defaultMinSyncPeriod is returned. Otherwise, the overwrite is returned.
func SyncPeriodOfShoot(respectSyncPeriodOverwrite bool, defaultMinSyncPeriod time.Duration, shoot *gardencorev1beta1.Shoot) time.Duration {
	if !RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot) {
		return defaultMinSyncPeriod
	}

	syncPeriodOverwrite, ok := shoot.Annotations[v1beta1constants.ShootSyncPeriod]
	if !ok {
		return defaultMinSyncPeriod
	}

	syncPeriod, err := time.ParseDuration(syncPeriodOverwrite)
	if err != nil {
		return defaultMinSyncPeriod
	}

	if syncPeriod < defaultMinSyncPeriod {
		return defaultMinSyncPeriod
	}
	return syncPeriod
}

// EffectiveMaintenanceTimeWindow cuts a maintenance time window at the end with a guess of 15 minutes. It is subtracted from the end
// of a maintenance time window to use a best-effort kind of finishing the operation before the end.
// Generally, we can't make sure that the maintenance operation is done by the end of the time window anyway (considering large
// clusters with hundreds of nodes, a rolling update will take several hours).
func EffectiveMaintenanceTimeWindow(timeWindow *timewindow.MaintenanceTimeWindow) *timewindow.MaintenanceTimeWindow {
	return timeWindow.WithEnd(timeWindow.End().Add(0, -15, 0))
}

// EffectiveShootMaintenanceTimeWindow returns the effective MaintenanceTimeWindow of the given Shoot.
func EffectiveShootMaintenanceTimeWindow(shoot *gardencorev1beta1.Shoot) *timewindow.MaintenanceTimeWindow {
	maintenance := shoot.Spec.Maintenance
	if maintenance == nil || maintenance.TimeWindow == nil {
		return timewindow.AlwaysTimeWindow
	}

	timeWindow, err := timewindow.ParseMaintenanceTimeWindow(maintenance.TimeWindow.Begin, maintenance.TimeWindow.End)
	if err != nil {
		return timewindow.AlwaysTimeWindow
	}

	return EffectiveMaintenanceTimeWindow(timeWindow)
}

// GetShootNameFromOwnerReferences attempts to get the name of the Shoot object which owns the passed in object.
// If it is not owned by a Shoot, an empty string is returned.
func GetShootNameFromOwnerReferences(objectMeta metav1.Object) string {
	for _, ownerRef := range objectMeta.GetOwnerReferences() {
		if ownerRef.Kind == "Shoot" {
			return ownerRef.Name
		}
	}
	return ""
}

// NodeLabelsForWorkerPool returns a combined map of all user-specified and gardener-managed node labels.
func NodeLabelsForWorkerPool(workerPool gardencorev1beta1.Worker, nodeLocalDNSEnabled bool, gardenerNodeAgentSecretName string) map[string]string {
	// copy worker pool labels map
	labels := utils.MergeStringMaps(workerPool.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	labels["node.kubernetes.io/role"] = "node"
	labels["kubernetes.io/arch"] = *workerPool.Machine.Architecture

	labels[v1beta1constants.LabelNodeLocalDNS] = strconv.FormatBool(nodeLocalDNSEnabled)

	if v1beta1helper.SystemComponentsAllowed(&workerPool) {
		labels[v1beta1constants.LabelWorkerPoolSystemComponents] = "true"
	}

	// worker pool name labels
	labels[v1beta1constants.LabelWorkerPool] = workerPool.Name
	labels[v1beta1constants.LabelWorkerPoolDeprecated] = workerPool.Name
	labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName] = gardenerNodeAgentSecretName

	// add CRI labels selected by the RuntimeClass
	if workerPool.CRI != nil {
		labels[extensionsv1alpha1.CRINameWorkerLabel] = string(workerPool.CRI.Name)
		if len(workerPool.CRI.ContainerRuntimes) > 0 {
			for _, cr := range workerPool.CRI.ContainerRuntimes {
				key := fmt.Sprintf(extensionsv1alpha1.ContainerRuntimeNameWorkerLabel, cr.Type)
				labels[key] = "true"
			}
		}
	}

	return labels
}

const (
	// ShootProjectSecretSuffixKubeconfig is a constant for a shoot project secret with suffix 'kubeconfig'.
	ShootProjectSecretSuffixKubeconfig = "kubeconfig"
	// ShootProjectSecretSuffixCACluster is a constant for a shoot project secret with suffix 'ca-cluster'.
	//
	// Deprecated: This constant is deprecated in favor of ShootProjectConfigMapSuffixCACluster
	ShootProjectSecretSuffixCACluster = "ca-cluster"
	// ShootProjectSecretSuffixCAClient is a constant for a shoot project secret with suffix 'ca-client'.
	ShootProjectSecretSuffixCAClient = "ca-client"
	// ShootProjectSecretSuffixSSHKeypair is a constant for a shoot project secret with suffix 'ssh-keypair'.
	ShootProjectSecretSuffixSSHKeypair = v1beta1constants.SecretNameSSHKeyPair
	// ShootProjectSecretSuffixOldSSHKeypair is a constant for a shoot project secret with suffix 'ssh-keypair.old'.
	ShootProjectSecretSuffixOldSSHKeypair = v1beta1constants.SecretNameSSHKeyPair + ".old"
	// ShootProjectSecretSuffixMonitoring is a constant for a shoot project secret with suffix 'monitoring'.
	ShootProjectSecretSuffixMonitoring = "monitoring"
	// ShootProjectConfigMapSuffixCACluster is a constant for a shoot project secret with suffix 'ca-cluster'.
	ShootProjectConfigMapSuffixCACluster = "ca-cluster"
	// ShootProjectConfigMapSuffixCAKubelet is a constant for a shoot project secret with suffix 'ca-kubelet'.
	ShootProjectConfigMapSuffixCAKubelet = "ca-kubelet"
)

// GetShootProjectSecretSuffixes returns the list of shoot-related project secret suffixes.
func GetShootProjectSecretSuffixes() []string {
	return []string{
		ShootProjectSecretSuffixKubeconfig,
		ShootProjectSecretSuffixCACluster,
		ShootProjectSecretSuffixSSHKeypair,
		ShootProjectSecretSuffixOldSSHKeypair,
		ShootProjectSecretSuffixMonitoring,
	}
}

// GetShootProjectInternalSecretSuffixes returns the list of shoot-related project internal secret suffixes.
func GetShootProjectInternalSecretSuffixes() []string {
	return []string{
		ShootProjectSecretSuffixCAClient,
	}
}

// GetShootProjectConfigMapSuffixes returns the list of shoot-related project config map suffixes.
func GetShootProjectConfigMapSuffixes() []string {
	return []string{
		ShootProjectConfigMapSuffixCACluster,
		ShootProjectConfigMapSuffixCAKubelet,
	}
}

func shootProjectResourceSuffix(suffix string) string {
	return "." + suffix
}

// ComputeShootProjectResourceName computes the name of a shoot-related project resource.
func ComputeShootProjectResourceName(shootName, suffix string) string {
	return shootName + shootProjectResourceSuffix(suffix)
}

// IsShootProjectSecret checks if the given name matches the name of a shoot-related project secret. If no, it returns
// an empty string and <false>. Otherwise, it returns the shoot name and <true>.
func IsShootProjectSecret(secretName string) (string, bool) {
	for _, v := range GetShootProjectSecretSuffixes() {
		if suffix := shootProjectResourceSuffix(v); strings.HasSuffix(secretName, suffix) {
			return strings.TrimSuffix(secretName, suffix), true
		}
	}

	return "", false
}

// IsShootProjectInternalSecret checks if the given name matches the name of a shoot-related project internal secret.
// If no, it returns an empty string and <false>. Otherwise, it returns the shoot name and <true>.
func IsShootProjectInternalSecret(secretName string) (string, bool) {
	for _, v := range GetShootProjectInternalSecretSuffixes() {
		if suffix := shootProjectResourceSuffix(v); strings.HasSuffix(secretName, suffix) {
			return strings.TrimSuffix(secretName, suffix), true
		}
	}

	return "", false
}

// ComputeManagedShootIssuerSecretName returns the name that should be used for
// storing the service account public keys of a shoot's kube-apiserver
// in the gardener-system-shoot-issuer namespace in the Garden cluster.
func ComputeManagedShootIssuerSecretName(projectName string, shootUID types.UID) string {
	return projectName + "--" + string(shootUID)
}

// IsShootProjectConfigMap checks if the given name matches the name of a shoot-related project config map. If no, it returns
// an empty string and <false>. Otherwise, it returns the shoot name and <true>.
func IsShootProjectConfigMap(configMapName string) (string, bool) {
	for _, v := range GetShootProjectConfigMapSuffixes() {
		if suffix := shootProjectResourceSuffix(v); strings.HasSuffix(configMapName, suffix) {
			return strings.TrimSuffix(configMapName, suffix), true
		}
	}

	return "", false
}

const (
	// SecretNamePrefixShootAccess is the prefix of all secrets containing credentials for accessing shoot clusters.
	SecretNamePrefixShootAccess = "shoot-access-"
	// VolumeMountPathGenericKubeconfig is a constant for the path to which the generic shoot kubeconfig will be mounted.
	VolumeMountPathGenericKubeconfig = "/var/run/secrets/gardener.cloud/shoot/generic-kubeconfig"
	// PathShootToken is a constant for the path at which the shoot token file is accessible.
	PathShootToken = VolumeMountPathGenericKubeconfig + "/" + resourcesv1alpha1.DataKeyToken
	// PathGenericKubeconfig is a constant for the path at which the kubeconfig file is accessible.
	PathGenericKubeconfig = VolumeMountPathGenericKubeconfig + "/" + secrets.DataKeyKubeconfig
)

// AccessSecret contains settings for a shoot/garden access secret consumed by a component communicating with a shoot
// or the garden API server.
type AccessSecret struct {
	Secret             *corev1.Secret
	ServiceAccountName string
	Class              string

	tokenExpirationDuration string
	kubeconfig              *clientcmdv1.Config
	targetSecretName        string
	targetSecretNamespace   string
	serviceAccountLabels    map[string]string
}

// NewShootAccessSecret returns a new AccessSecret object and initializes it with an empty corev1.Secret object
// with the given name and namespace. If not already done, the name will be prefixed with the
// SecretNamePrefixShootAccess. The ServiceAccountName field will be defaulted with the name.
func NewShootAccessSecret(name, namespace string) *AccessSecret {
	if !strings.HasPrefix(name, SecretNamePrefixShootAccess) {
		name = SecretNamePrefixShootAccess + name
	}

	return &AccessSecret{
		Secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
		ServiceAccountName: strings.TrimPrefix(name, SecretNamePrefixShootAccess),
		Class:              resourcesv1alpha1.ResourceManagerClassShoot,
	}
}

// WithNameOverride sets the ObjectMeta.Name field of the *corev1.Secret inside the AccessSecret.
func (s *AccessSecret) WithNameOverride(name string) *AccessSecret {
	s.Secret.Name = name
	return s
}

// WithNamespaceOverride sets the ObjectMeta.Namespace field of the *corev1.Secret inside the AccessSecret.
func (s *AccessSecret) WithNamespaceOverride(namespace string) *AccessSecret {
	s.Secret.Namespace = namespace
	return s
}

// WithServiceAccountName sets the ServiceAccountName field of the AccessSecret.
func (s *AccessSecret) WithServiceAccountName(name string) *AccessSecret {
	s.ServiceAccountName = name
	return s
}

// WithServiceAccountLabels sets the serviceAccountLabels field of the AccessSecret.
func (s *AccessSecret) WithServiceAccountLabels(labels map[string]string) *AccessSecret {
	s.serviceAccountLabels = labels
	return s
}

// WithTokenExpirationDuration sets the tokenExpirationDuration field of the AccessSecret.
func (s *AccessSecret) WithTokenExpirationDuration(duration string) *AccessSecret {
	s.tokenExpirationDuration = duration
	return s
}

// WithKubeconfig sets the kubeconfig field of the AccessSecret.
func (s *AccessSecret) WithKubeconfig(kubeconfigRaw *clientcmdv1.Config) *AccessSecret {
	s.kubeconfig = kubeconfigRaw
	return s
}

// WithTargetSecret sets the kubeconfig field of the AccessSecret.
func (s *AccessSecret) WithTargetSecret(name, namespace string) *AccessSecret {
	s.targetSecretName = name
	s.targetSecretNamespace = namespace
	return s
}

// Reconcile creates or patches the given shoot access secret. Based on the struct configuration, it adds the required
// annotations for the token requestor controller of gardener-resource-manager.
func (s *AccessSecret) Reconcile(ctx context.Context, c client.Client) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, s.Secret, func() error {
		s.Secret.Type = corev1.SecretTypeOpaque
		metav1.SetMetaDataLabel(&s.Secret.ObjectMeta, resourcesv1alpha1.ResourceManagerPurpose, resourcesv1alpha1.LabelPurposeTokenRequest)
		metav1.SetMetaDataLabel(&s.Secret.ObjectMeta, resourcesv1alpha1.ResourceManagerClass, s.Class)
		metav1.SetMetaDataAnnotation(&s.Secret.ObjectMeta, resourcesv1alpha1.ServiceAccountName, s.ServiceAccountName)

		if s.Class == resourcesv1alpha1.ResourceManagerClassShoot {
			metav1.SetMetaDataAnnotation(&s.Secret.ObjectMeta, resourcesv1alpha1.ServiceAccountNamespace, metav1.NamespaceSystem)
		}

		if s.serviceAccountLabels != nil {
			labelsJSON, err := json.Marshal(s.serviceAccountLabels)
			if err != nil {
				return fmt.Errorf("failed marshaling the service account labels to JSON: %w", err)
			}
			metav1.SetMetaDataAnnotation(&s.Secret.ObjectMeta, resourcesv1alpha1.ServiceAccountLabels, string(labelsJSON))
		}

		if s.tokenExpirationDuration != "" {
			metav1.SetMetaDataAnnotation(&s.Secret.ObjectMeta, resourcesv1alpha1.ServiceAccountTokenExpirationDuration, s.tokenExpirationDuration)
		}

		if s.targetSecretName != "" {
			metav1.SetMetaDataAnnotation(&s.Secret.ObjectMeta, resourcesv1alpha1.TokenRequestorTargetSecretName, s.targetSecretName)
		}

		if s.targetSecretNamespace != "" {
			metav1.SetMetaDataAnnotation(&s.Secret.ObjectMeta, resourcesv1alpha1.TokenRequestorTargetSecretNamespace, s.targetSecretNamespace)
		}

		if s.kubeconfig == nil {
			delete(s.Secret.Data, resourcesv1alpha1.DataKeyKubeconfig)
		} else {
			delete(s.Secret.Data, resourcesv1alpha1.DataKeyToken)

			if kubeconfigRaw, ok := s.Secret.Data[resourcesv1alpha1.DataKeyKubeconfig]; ok {
				existingKubeconfig := &clientcmdv1.Config{}
				if _, _, err := clientcmdlatest.Codec.Decode(kubeconfigRaw, nil, existingKubeconfig); err != nil {
					return err
				}
				s.kubeconfig.AuthInfos = existingKubeconfig.AuthInfos
			}

			kubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, s.kubeconfig)
			if err != nil {
				return err
			}

			if s.Secret.Data == nil {
				s.Secret.Data = make(map[string][]byte, 1)
			}

			s.Secret.Data[resourcesv1alpha1.DataKeyKubeconfig] = kubeconfigRaw
		}

		return nil
	},
		// The token-requestor might concurrently update the kubeconfig secret key to populate the token.
		// Hence, we need to use optimistic locking here to ensure we don't accidentally overwrite the concurrent update.
		// ref https://github.com/gardener/gardener/issues/6092#issuecomment-1156244514
		controllerutils.MergeFromOption{MergeFromOption: client.MergeFromWithOptimisticLock{}})
	return err
}

// InjectGenericKubeconfig injects the volumes and volume mounts for the generic shoot kubeconfig into the provided
// object. The access secret name must be the name of a secret containing a JWT token which should be used by the
// kubeconfig. If the object has multiple containers then the default is to inject it into all of them. If it should
// only be done for a selection of containers then their respective names must be provided.
func InjectGenericKubeconfig(obj runtime.Object, genericKubeconfigName, accessSecretName string, containerNames ...string) error {
	return injectGenericKubeconfig(obj, genericKubeconfigName, accessSecretName, "kubeconfig", VolumeMountPathGenericKubeconfig, containerNames...)
}

func injectGenericKubeconfig(obj runtime.Object, genericKubeconfigName, accessSecretName, volumeName, mountPath string, containerNames ...string) error {
	var (
		volume = corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: ptr.To[int32](420),
					Sources: []corev1.VolumeProjection{
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: genericKubeconfigName,
								},
								Items: []corev1.KeyToPath{{
									Key:  secrets.DataKeyKubeconfig,
									Path: secrets.DataKeyKubeconfig,
								}},
								Optional: ptr.To(false),
							},
						},
						{
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: accessSecretName,
								},
								Items: []corev1.KeyToPath{{
									Key:  resourcesv1alpha1.DataKeyToken,
									Path: resourcesv1alpha1.DataKeyToken,
								}},
								Optional: ptr.To(false),
							},
						},
					},
				},
			},
		}

		volumeMount = corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: mountPath,
			ReadOnly:  true,
		}
	)

	return kubernetesutils.VisitPodSpec(obj, func(podSpec *corev1.PodSpec) {
		kubernetesutils.AddVolume(podSpec, volume, true)
		kubernetesutils.VisitContainers(podSpec, func(container *corev1.Container) {
			kubernetesutils.AddVolumeMount(container, volumeMount, true)
		}, containerNames...)
	})
}

// GetShootSeedNames returns the spec.seedName and the status.seedName field in case the provided object is a Shoot.
func GetShootSeedNames(obj client.Object) (*string, *string) {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return nil, nil
	}
	return shoot.Spec.SeedName, shoot.Status.SeedName
}

// ExtractSystemComponentsTolerations returns tolerations that are required to schedule shoot system components
// on the given workers. Tolerations are only considered for workers which have `SystemComponents.Allow: true`.
func ExtractSystemComponentsTolerations(workers []gardencorev1beta1.Worker) []corev1.Toleration {
	var (
		tolerations = sets.New[corev1.Toleration]()

		// We need to use semantically equal tolerations, i.e. equality of underlying values of pointers,
		// before they are added to the tolerations set.
		comparableTolerations = &kubernetesutils.ComparableTolerations{}
	)

	for _, worker := range workers {
		if worker.ControlPlane != nil {
			tolerations.Insert(corev1.Toleration{Key: "node-role.kubernetes.io/control-plane", Operator: corev1.TolerationOpExists})
		}

		if v1beta1helper.SystemComponentsAllowed(&worker) {
			for _, taint := range worker.Taints {
				toleration := kubernetesutils.TolerationForTaint(taint)
				tolerations.Insert(comparableTolerations.Transform(toleration))
			}
		}
	}

	sortedTolerations := tolerations.UnsortedList()

	// sort system component tolerations for a stable output
	slices.SortFunc(sortedTolerations, func(a, b corev1.Toleration) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return sortedTolerations
}

// IncompleteDNSConfigError is a custom error type.
type IncompleteDNSConfigError struct{}

// Error prints the error message of the IncompleteDNSConfigError error.
func (e *IncompleteDNSConfigError) Error() string {
	return "unable to figure out which secret should be used for dns"
}

// IsIncompleteDNSConfigError returns true if the error indicates that not the DNS config is incomplete.
func IsIncompleteDNSConfigError(err error) bool {
	_, ok := err.(*IncompleteDNSConfigError)
	return ok
}

// ConstructInternalClusterDomain constructs the internal base domain for this shoot cluster.
// It is only used for internal purposes (all kubeconfigs except the one which is received by the
// user will only talk with the kube-apiserver via a DNS record of domain). In case the given <internalDomain>
// already contains "internal", the result is constructed as "<shootName>.<shootProject>.<internalDomain>."
// In case it does not, the word "internal" will be appended, resulting in
// "<shootName>.<shootProject>.internal.<internalDomain>".
func ConstructInternalClusterDomain(shootName, shootProject string, internalDomain *Domain) string {
	if internalDomain == nil {
		return ""
	}
	if strings.Contains(internalDomain.Domain, InternalDomainKey) {
		return fmt.Sprintf("%s.%s.%s", shootName, shootProject, internalDomain.Domain)
	}
	return fmt.Sprintf("%s.%s.%s.%s", shootName, shootProject, InternalDomainKey, internalDomain.Domain)
}

// ConstructExternalClusterDomain constructs the external Shoot cluster domain, i.e. the domain which will be put
// into the Kubeconfig handed out to the user.
func ConstructExternalClusterDomain(shoot *gardencorev1beta1.Shoot) *string {
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		return nil
	}
	return shoot.Spec.DNS.Domain
}

// ConstructExternalDomain constructs an object containing all relevant information of the external domain that
// shall be used for a shoot cluster - based on the configuration of the Garden cluster and the shoot itself.
// Shoot credentials should be of type [*corev1.Secret] or [*securityv1alpha1.WorkloadIdentity].
func ConstructExternalDomain(ctx context.Context, c client.Reader, shoot *gardencorev1beta1.Shoot, shootCredentials client.Object, defaultDomains []*Domain) (*Domain, error) {
	externalClusterDomain := ConstructExternalClusterDomain(shoot)
	if externalClusterDomain == nil {
		return nil, nil
	}

	var (
		externalDomain  = &Domain{Domain: *shoot.Spec.DNS.Domain}
		defaultDomain   = DomainIsDefaultDomain(*externalClusterDomain, defaultDomains)
		primaryProvider = v1beta1helper.FindPrimaryDNSProvider(shoot.Spec.DNS.Providers)
	)

	switch {
	case defaultDomain != nil:
		externalDomain.SecretData = defaultDomain.SecretData
		externalDomain.Provider = defaultDomain.Provider
		externalDomain.Zone = defaultDomain.Zone

	case primaryProvider != nil:
		if primaryProvider.SecretName != nil {
			secret := &corev1.Secret{}
			if err := c.Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: *primaryProvider.SecretName}, secret); err != nil {
				return nil, fmt.Errorf("could not get dns provider secret %q: %+v", *shoot.Spec.DNS.Providers[0].SecretName, err)
			}
			externalDomain.SecretData = secret.Data
		} else {
			if shootCredentials == nil {
				return nil, fmt.Errorf("default domain is not present, secret for primary dns provider is required")
			}
			switch creds := shootCredentials.(type) {
			case *corev1.Secret:
				externalDomain.SecretData = creds.Data
			case *securityv1alpha1.WorkloadIdentity:
				// TODO(dimityrmirchev): This code should eventually handle shoot credentials being of type WorkloadIdentity
				return nil, fmt.Errorf("shoot credentials of type WorkloadIdentity cannot be used as domain secret")
			default:
				return nil, fmt.Errorf("unexpected shoot credentials type")
			}
		}
		if primaryProvider.Type != nil {
			externalDomain.Provider = *primaryProvider.Type
		}
		if zones := primaryProvider.Zones; zones != nil {
			if len(zones.Include) == 1 {
				externalDomain.Zone = zones.Include[0]
			}
		}

	default:
		return nil, &IncompleteDNSConfigError{}
	}

	return externalDomain, nil
}

// ComputeRequiredExtensionsForShoot computes the extension kind/type combinations that are required for the
// shoot reconciliation flow.
func ComputeRequiredExtensionsForShoot(shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed, controllerRegistrationList *gardencorev1beta1.ControllerRegistrationList, internalDomain, externalDomain *Domain) sets.Set[string] {
	requiredExtensions := sets.New[string]()

	if backupConfig := v1beta1helper.GetBackupConfigForShoot(shoot, seed); backupConfig != nil {
		requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.BackupBucketResource, backupConfig.Provider))
		requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.BackupEntryResource, backupConfig.Provider))
	}

	if seed != nil {
		// Hint: This is actually a temporary work-around to request the control plane extension of the seed provider type as
		// it might come with webhooks that are configuring the exposure of shoot control planes. The ControllerRegistration resource
		// does not reflect this today.
		requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.ControlPlaneResource, seed.Spec.Provider.Type))
	}

	if !v1beta1helper.IsWorkerless(shoot) {
		requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.ControlPlaneResource, shoot.Spec.Provider.Type))
		requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.InfrastructureResource, shoot.Spec.Provider.Type))
		requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.WorkerResource, shoot.Spec.Provider.Type))

		if shoot.Spec.Networking != nil && shoot.Spec.Networking.Type != nil {
			requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.NetworkResource, *shoot.Spec.Networking.Type))
		}
	}

	disabledExtensions := sets.New[string]()
	for _, extension := range shoot.Spec.Extensions {
		id := ExtensionsID(extensionsv1alpha1.ExtensionResource, extension.Type)

		if ptr.Deref(extension.Disabled, false) {
			disabledExtensions.Insert(id)
		} else {
			requiredExtensions.Insert(id)
		}
	}

	for _, pool := range shoot.Spec.Provider.Workers {
		if pool.Machine.Image != nil {
			requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.OperatingSystemConfigResource, pool.Machine.Image.Name))
		}
		if pool.CRI != nil {
			for _, cr := range pool.CRI.ContainerRuntimes {
				requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.ContainerRuntimeResource, cr.Type))
			}
		}
	}

	if shoot.Spec.DNS != nil {
		for _, provider := range shoot.Spec.DNS.Providers {
			if provider.Type != nil && *provider.Type != core.DNSUnmanaged {
				if provider.Primary != nil && *provider.Primary {
					requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.DNSRecordResource, *provider.Type))
				}
			}
		}
	}

	if internalDomain != nil && internalDomain.Provider != core.DNSUnmanaged {
		requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.DNSRecordResource, internalDomain.Provider))
	}

	if externalDomain != nil && externalDomain.Provider != core.DNSUnmanaged {
		requiredExtensions.Insert(ExtensionsID(extensionsv1alpha1.DNSRecordResource, externalDomain.Provider))
	}

	for _, controllerRegistration := range controllerRegistrationList.Items {
		for _, resource := range controllerRegistration.Spec.Resources {
			id := ExtensionsID(extensionsv1alpha1.ExtensionResource, resource.Type)
			if resource.Kind == extensionsv1alpha1.ExtensionResource && ptr.Deref(resource.GloballyEnabled, false) && !disabledExtensions.Has(id) {
				if v1beta1helper.IsWorkerless(shoot) && !ptr.Deref(resource.WorkerlessSupported, false) {
					continue
				}
				requiredExtensions.Insert(id)
			}
		}
	}

	return requiredExtensions
}

// ExtensionsID returns an identifier for the given extension kind/type.
func ExtensionsID(extensionKind, extensionType string) string {
	return fmt.Sprintf("%s/%s", extensionKind, extensionType)
}

// ComputeTechnicalID determines the technical id of the given Shoot which is later used for the name of the
// namespace and for tagging all the resources created in the infrastructure.
func ComputeTechnicalID(projectName string, shoot *gardencorev1beta1.Shoot) string {
	// Use the stored technical ID in the Shoot's status field if it's there.
	// For backwards compatibility we keep the pattern as it was before we had to change it
	// (double hyphens).
	if len(shoot.Status.TechnicalID) > 0 {
		return shoot.Status.TechnicalID
	}

	// New clusters shall be created with the new technical id (double hyphens).
	return fmt.Sprintf("%s-%s--%s", v1beta1constants.TechnicalIDPrefix, projectName, shoot.Name)
}

// IsShootNamespace returns true if the given namespace is a shoot namespace, i.e. it starts with the technical id prefix.
func IsShootNamespace(namespace string) bool {
	return strings.HasPrefix(namespace, v1beta1constants.TechnicalIDPrefix)
}

// GetShootConditionTypes returns all known shoot condition types.
func GetShootConditionTypes(workerless bool) []gardencorev1beta1.ConditionType {
	shootConditionTypes := []gardencorev1beta1.ConditionType{
		gardencorev1beta1.ShootAPIServerAvailable,
		gardencorev1beta1.ShootControlPlaneHealthy,
		gardencorev1beta1.ShootObservabilityComponentsHealthy,
	}

	if !workerless {
		shootConditionTypes = append(shootConditionTypes, gardencorev1beta1.ShootEveryNodeReady)
	}

	return append(shootConditionTypes, gardencorev1beta1.ShootSystemComponentsHealthy)
}

// DefaultGVKsForEncryption returns the list of GroupVersionKinds which are encrypted by default.
func DefaultGVKsForEncryption() []schema.GroupVersionKind {
	return []schema.GroupVersionKind{
		corev1.SchemeGroupVersion.WithKind("Secret"),
	}
}

// DefaultResourcesForEncryption returns the list of resources which are encrypted by default.
func DefaultResourcesForEncryption() sets.Set[string] {
	return sets.New(corev1.Resource("secrets").String())
}

// GetIPStackForShoot returns the value for the AnnotationKeyIPStack annotation based on the given shoot.
// It falls back to IPv4 if no IP families are available, e.g. in a workerless shoot cluster.
func GetIPStackForShoot(shoot *gardencorev1beta1.Shoot) string {
	var ipFamilies []gardencorev1beta1.IPFamily
	if networking := shoot.Spec.Networking; networking != nil {
		ipFamilies = networking.IPFamilies
	}
	return getIPStackForFamilies(ipFamilies)
}

// CalculateDataStringForKubeletConfiguration returns a data string for the relevant fields of the kubelet configuration.
func CalculateDataStringForKubeletConfiguration(kubeletConfiguration *gardencorev1beta1.KubeletConfig) []string {
	var data []string

	if kubeletConfiguration == nil {
		return nil
	}

	if resources := v1beta1helper.SumResourceReservations(kubeletConfiguration.KubeReserved, kubeletConfiguration.SystemReserved); resources != nil {
		data = append(data, fmt.Sprintf("%s-%s-%s-%s", resources.CPU, resources.Memory, resources.PID, resources.EphemeralStorage))
	}
	if eviction := kubeletConfiguration.EvictionHard; eviction != nil {
		data = append(data, fmt.Sprintf("%s-%s-%s-%s-%s",
			ptr.Deref(eviction.ImageFSAvailable, ""),
			ptr.Deref(eviction.ImageFSInodesFree, ""),
			ptr.Deref(eviction.MemoryAvailable, ""),
			ptr.Deref(eviction.NodeFSAvailable, ""),
			ptr.Deref(eviction.NodeFSInodesFree, ""),
		))
	}

	if policy := kubeletConfiguration.CPUManagerPolicy; policy != nil {
		data = append(data, *policy)
	}

	return data
}

// IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled checks if the feature gate "MatchLabelKeysInPodTopologySpread" is disabled in
// both kube-apiserver and kube-scheduler in the Shoot.
func IsMatchLabelKeysInPodTopologySpreadFeatureGateDisabled(shoot *gardencorev1beta1.Shoot) bool {
	if shoot == nil || shoot.Spec.Kubernetes.KubeAPIServer == nil || shoot.Spec.Kubernetes.KubeScheduler == nil {
		return false
	}

	valueKubeAPIServer, ok := shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates["MatchLabelKeysInPodTopologySpread"]
	if !ok {
		return false
	}

	valueKubeScheduler, ok := shoot.Spec.Kubernetes.KubeScheduler.FeatureGates["MatchLabelKeysInPodTopologySpread"]
	if !ok {
		return false
	}

	return !valueKubeAPIServer && !valueKubeScheduler
}

// IsAuthorizeWithSelectorsEnabled checks if the feature gate "AuthorizeWithSelectors" is enabled in the kube-apiserver
// of the Shoot.
func IsAuthorizeWithSelectorsEnabled(kubeAPIServer *gardencorev1beta1.KubeAPIServerConfig, kubernetesVersion *semver.Version) bool {
	// The feature gate is beta in v1.32 and enabled by default.
	if versionutils.ConstraintK8sGreaterEqual132.Check(kubernetesVersion) {
		if kubeAPIServer == nil {
			return true
		}

		value, ok := kubeAPIServer.FeatureGates["AuthorizeWithSelectors"]
		if !ok {
			return true
		}

		return value
	}

	// The feature gate is alpha in v1.31 and disabled by default.
	if versionutils.ConstraintK8sGreaterEqual131.Check(kubernetesVersion) {
		if kubeAPIServer == nil {
			return false
		}

		value, ok := kubeAPIServer.FeatureGates["AuthorizeWithSelectors"]
		if !ok {
			return false
		}

		return value
	}

	return false
}

// CalculateWorkerPoolHashForInPlaceUpdate calculates the data string for the worker pool hash to be used for in-place updates.
//
// WARNING: Changing this function will cause an in-place update of all the existing nodes. Use with caution.
func CalculateWorkerPoolHashForInPlaceUpdate(workerPoolName string, kubernetesVersion *string, kubeletConfig *gardencorev1beta1.KubeletConfig, machineImageVersion string, credentials *gardencorev1beta1.ShootCredentials) (string, error) {
	var data []string

	kubernetesSemverVersion, err := semver.NewVersion(ptr.Deref(kubernetesVersion, ""))
	if err != nil {
		return "", fmt.Errorf("failed to parse kubernetes version %s: %w", ptr.Deref(kubernetesVersion, ""), err)
	}
	kubernetesMajorMinorVersion := fmt.Sprintf("%d.%d", kubernetesSemverVersion.Major(), kubernetesSemverVersion.Minor())

	data = append(data, kubernetesMajorMinorVersion, machineImageVersion)

	if credentials != nil && credentials.Rotation != nil {
		if credentials.Rotation.CertificateAuthorities != nil {
			if lastInitiationTime := v1beta1helper.LastInitiationTimeForWorkerPool(workerPoolName, credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, credentials.Rotation.CertificateAuthorities.LastInitiationTime); lastInitiationTime != nil {
				data = append(data, lastInitiationTime.String())
			}
		}
		if credentials.Rotation.ServiceAccountKey != nil {
			if lastInitiationTime := v1beta1helper.LastInitiationTimeForWorkerPool(workerPoolName, credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, credentials.Rotation.ServiceAccountKey.LastInitiationTime); lastInitiationTime != nil {
				data = append(data, lastInitiationTime.String())
			}
		}
	}

	data = append(data, CalculateDataStringForKubeletConfiguration(kubeletConfig)...)

	var result string
	for _, v := range data {
		result += utils.ComputeSHA256Hex([]byte(v))
	}

	return utils.ComputeSHA256Hex([]byte(result))[:16], nil
}
