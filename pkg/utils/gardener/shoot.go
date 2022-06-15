// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

// RespectShootSyncPeriodOverwrite checks whether to respect the sync period overwrite of a Shoot or not.
func RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite bool, shoot *v1beta1.Shoot) bool {
	return respectSyncPeriodOverwrite || shoot.Namespace == v1beta1constants.GardenNamespace
}

// ShouldIgnoreShoot determines whether a Shoot should be ignored or not.
func ShouldIgnoreShoot(respectSyncPeriodOverwrite bool, shoot *v1beta1.Shoot) bool {
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

// IsShootFailed checks if a Shoot is failed.
func IsShootFailed(shoot *v1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation

	return lastOperation != nil && lastOperation.State == v1beta1.LastOperationStateFailed &&
		shoot.Generation == shoot.Status.ObservedGeneration &&
		shoot.Status.Gardener.Version == version.Get().GitVersion
}

// IsNowInEffectiveShootMaintenanceTimeWindow checks if the current time is in the effective
// maintenance time window of the Shoot.
func IsNowInEffectiveShootMaintenanceTimeWindow(shoot *v1beta1.Shoot) bool {
	return EffectiveShootMaintenanceTimeWindow(shoot).Contains(time.Now())
}

// LastReconciliationDuringThisTimeWindow returns true if <now> is contained in the given effective maintenance time
// window of the shoot and if the <lastReconciliation> did not happen longer than the longest possible duration of a
// maintenance time window.
func LastReconciliationDuringThisTimeWindow(shoot *v1beta1.Shoot) bool {
	if shoot.Status.LastOperation == nil {
		return false
	}

	var (
		timeWindow         = EffectiveShootMaintenanceTimeWindow(shoot)
		now                = time.Now()
		lastReconciliation = shoot.Status.LastOperation.LastUpdateTime.Time
	)

	return timeWindow.Contains(lastReconciliation) && now.UTC().Sub(lastReconciliation.UTC()) <= v1beta1.MaintenanceTimeWindowDurationMaximum
}

// IsObservedAtLatestGenerationAndSucceeded checks whether the Shoot's generation has changed or if the LastOperation status
// is Succeeded.
func IsObservedAtLatestGenerationAndSucceeded(shoot *v1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration &&
		(lastOperation != nil && lastOperation.State == v1beta1.LastOperationStateSucceeded)
}

// SyncPeriodOfShoot determines the sync period of the given shoot.
//
// If no overwrite is allowed, the defaultMinSyncPeriod is returned.
// Otherwise, the overwrite is parsed. If an error occurs or it is smaller than the defaultMinSyncPeriod,
// the defaultMinSyncPeriod is returned. Otherwise, the overwrite is returned.
func SyncPeriodOfShoot(respectSyncPeriodOverwrite bool, defaultMinSyncPeriod time.Duration, shoot *v1beta1.Shoot) time.Duration {
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
func EffectiveShootMaintenanceTimeWindow(shoot *v1beta1.Shoot) *timewindow.MaintenanceTimeWindow {
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

const (
	// ShootProjectSecretSuffixKubeconfig is a constant for a shoot project secret with suffix 'kubeconfig'.
	ShootProjectSecretSuffixKubeconfig = "kubeconfig"
	// ShootProjectSecretSuffixCACluster is a constant for a shoot project secret with suffix 'ca-cluster'.
	ShootProjectSecretSuffixCACluster = "ca-cluster"
	// ShootProjectSecretSuffixSSHKeypair is a constant for a shoot project secret with suffix 'ssh-keypair'.
	ShootProjectSecretSuffixSSHKeypair = v1beta1constants.SecretNameSSHKeyPair
	// ShootProjectSecretSuffixOldSSHKeypair is a constant for a shoot project secret with suffix 'ssh-keypair.old'.
	ShootProjectSecretSuffixOldSSHKeypair = v1beta1constants.SecretNameSSHKeyPair + ".old"
	// ShootProjectSecretSuffixMonitoring is a constant for a shoot project secret with suffix 'monitoring'.
	ShootProjectSecretSuffixMonitoring = "monitoring"
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

func shootProjectSecretSuffix(suffix string) string {
	return "." + suffix
}

// ComputeShootProjectSecretName computes the name of a shoot-related project secret.
func ComputeShootProjectSecretName(shootName, suffix string) string {
	return shootName + shootProjectSecretSuffix(suffix)
}

// IsShootProjectSecret checks if the given name matches the name of a shoot-related project secret. If no, it returns
// an empty string and <false>. Otherwise, it returns the shoot name and <true>.
func IsShootProjectSecret(secretName string) (string, bool) {
	for _, v := range GetShootProjectSecretSuffixes() {
		if suffix := shootProjectSecretSuffix(v); strings.HasSuffix(secretName, suffix) {
			return strings.TrimSuffix(secretName, suffix), true
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

// ShootAccessSecret contains settings for a shoot access secret consumed by a component communicating with a shoot API
// server.
type ShootAccessSecret struct {
	Secret             *corev1.Secret
	ServiceAccountName string

	tokenExpirationDuration string
	kubeconfig              *clientcmdv1.Config
	targetSecretName        string
	targetSecretNamespace   string
}

// NewShootAccessSecret returns a new ShootAccessSecret object and initializes it with an empty corev1.Secret object
// with for the given name and namespace. If not already done, the name will be prefixed with the
// SecretNamePrefixShootAccess. The ServiceAccountName field will be defaulted with the name.
func NewShootAccessSecret(name, namespace string) *ShootAccessSecret {
	if !strings.HasPrefix(name, SecretNamePrefixShootAccess) {
		name = SecretNamePrefixShootAccess + name
	}

	return &ShootAccessSecret{
		Secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
		ServiceAccountName: strings.TrimPrefix(name, SecretNamePrefixShootAccess),
	}
}

// WithNameOverride sets the ObjectMeta.Name field of the *corev1.Secret inside the ShootAccessSecret.
func (s *ShootAccessSecret) WithNameOverride(name string) *ShootAccessSecret {
	s.Secret.Name = name
	return s
}

// WithNamespaceOverride sets the ObjectMeta.Namespace field of the *corev1.Secret inside the ShootAccessSecret.
func (s *ShootAccessSecret) WithNamespaceOverride(namespace string) *ShootAccessSecret {
	s.Secret.Namespace = namespace
	return s
}

// WithServiceAccountName sets the ServiceAccountName field of the ShootAccessSecret.
func (s *ShootAccessSecret) WithServiceAccountName(name string) *ShootAccessSecret {
	s.ServiceAccountName = name
	return s
}

// WithTokenExpirationDuration sets the tokenExpirationDuration field of the ShootAccessSecret.
func (s *ShootAccessSecret) WithTokenExpirationDuration(duration string) *ShootAccessSecret {
	s.tokenExpirationDuration = duration
	return s
}

// WithKubeconfig sets the kubeconfig field of the ShootAccessSecret.
func (s *ShootAccessSecret) WithKubeconfig(kubeconfigRaw *clientcmdv1.Config) *ShootAccessSecret {
	s.kubeconfig = kubeconfigRaw
	return s
}

// WithTargetSecret sets the kubeconfig field of the ShootAccessSecret.
func (s *ShootAccessSecret) WithTargetSecret(name, namespace string) *ShootAccessSecret {
	s.targetSecretName = name
	s.targetSecretNamespace = namespace
	return s
}

// Reconcile creates or patches the given shoot access secret. Based on the struct configuration, it adds the required
// annotations for the token requestor controller of gardener-resource-manager.
func (s *ShootAccessSecret) Reconcile(ctx context.Context, c client.Client) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, s.Secret, func() error {
		s.Secret.Type = corev1.SecretTypeOpaque
		metav1.SetMetaDataLabel(&s.Secret.ObjectMeta, resourcesv1alpha1.ResourceManagerPurpose, resourcesv1alpha1.LabelPurposeTokenRequest)
		metav1.SetMetaDataAnnotation(&s.Secret.ObjectMeta, resourcesv1alpha1.ServiceAccountName, s.ServiceAccountName)
		metav1.SetMetaDataAnnotation(&s.Secret.ObjectMeta, resourcesv1alpha1.ServiceAccountNamespace, metav1.NamespaceSystem)

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
		client.MergeFromWithOptimisticLock{})
	return err
}

// InjectGenericKubeconfig injects the volumes and volume mounts for the generic shoot kubeconfig into the provided
// object. The access secret name must be the name of a secret containing a JWT token which should be used by the
// kubeconfig. If the object has multiple containers then the default is to inject it into all of them. If it should
// only be done for a selection of containers then their respective names must be provided.
func InjectGenericKubeconfig(obj runtime.Object, genericKubeconfigName, accessSecretName string, containerNames ...string) error {
	switch o := obj.(type) {
	case *corev1.Pod:
		injectGenericKubeconfig(&o.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *appsv1.Deployment:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *appsv1beta2.Deployment:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *appsv1beta1.Deployment:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *appsv1.StatefulSet:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *appsv1beta2.StatefulSet:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *appsv1beta1.StatefulSet:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *appsv1.DaemonSet:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *appsv1beta2.DaemonSet:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *batchv1.Job:
		injectGenericKubeconfig(&o.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *batchv1.CronJob:
		injectGenericKubeconfig(&o.Spec.JobTemplate.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	case *batchv1beta1.CronJob:
		injectGenericKubeconfig(&o.Spec.JobTemplate.Spec.Template.Spec, genericKubeconfigName, accessSecretName, containerNames...)

	default:
		return fmt.Errorf("unhandled object type %T", obj)
	}

	return nil
}

func injectGenericKubeconfig(podSpec *corev1.PodSpec, genericKubeconfigName, accessSecretName string, containerNames ...string) {
	var (
		volume = corev1.Volume{
			Name: "kubeconfig",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					DefaultMode: pointer.Int32(420),
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
								Optional: pointer.Bool(false),
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
								Optional: pointer.Bool(false),
							},
						},
					},
				},
			},
		}

		volumeMount = corev1.VolumeMount{
			Name:      volume.Name,
			MountPath: VolumeMountPathGenericKubeconfig,
			ReadOnly:  true,
		}
	)

	podSpec.Volumes = append(podSpec.Volumes, volume)
	for i, container := range podSpec.Containers {
		if len(containerNames) == 0 || utils.ValueExists(container.Name, containerNames) {
			podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, volumeMount)
		}
	}
}
