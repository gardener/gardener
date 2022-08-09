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

package etcd

import (
	"context"
	"fmt"
	"strconv"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// Druid is a constant for the name of the etcd-druid.
	Druid = "etcd-druid"

	druidRBACName                                = "gardener.cloud:system:" + Druid
	druidServiceAccountName                      = Druid
	druidVPAName                                 = Druid + "-vpa"
	druidConfigMapImageVectorOverwriteNamePrefix = Druid + "-imagevector-overwrite"
	druidDeploymentName                          = Druid
	managedResourceControlName                   = Druid

	druidConfigMapImageVectorOverwriteDataKey          = "images_overwrite.yaml"
	druidDeploymentVolumeMountPathImageVectorOverwrite = "/charts_overwrite"
	druidDeploymentVolumeNameImageVectorOverwrite      = "imagevector-overwrite"
)

// NewBootstrapper creates a new instance of DeployWaiter for the etcd bootstrapper.
func NewBootstrapper(c client.Client, namespace string, config *config.GardenletConfiguration, image string, imageVectorOverwrite *string) component.DeployWaiter {
	return &bootstrapper{
		client:               c,
		namespace:            namespace,
		config:               config,
		image:                image,
		imageVectorOverwrite: imageVectorOverwrite,
	}
}

type bootstrapper struct {
	client               client.Client
	namespace            string
	config               *config.GardenletConfiguration
	image                string
	imageVectorOverwrite *string
}

func (b *bootstrapper) Deploy(ctx context.Context) error {
	var (
		registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
		labels   = func() map[string]string { return map[string]string{v1beta1constants.GardenRole: Druid} }

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidServiceAccountName,
				Namespace: b.namespace,
				Labels:    labels(),
			},
			AutomountServiceAccountToken: pointer.Bool(false),
		}

		clusterRole = &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:   druidRBACName,
				Labels: labels(),
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list", "watch", "delete"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"secrets", "endpoints"},
					Verbs:     []string{"get", "list", "patch", "update", "watch"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"events"},
					Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"serviceaccounts"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{rbacv1.GroupName},
					Resources: []string{"roles", "rolebindings"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{corev1.GroupName, appsv1.GroupName},
					Resources: []string{"services", "configmaps", "statefulsets"},
					Verbs:     []string{"get", "list", "patch", "update", "watch", "create", "delete"},
				},
				{
					APIGroups: []string{batchv1.GroupName},
					Resources: []string{"jobs"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{batchv1beta1.GroupName},
					Resources: []string{"cronjobs"},
					Verbs:     []string{"get", "list", "watch", "delete"},
				},
				{
					APIGroups: []string{druidv1alpha1.GroupVersion.Group},
					Resources: []string{"etcds", "etcdcopybackupstasks"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
				{
					APIGroups: []string{druidv1alpha1.GroupVersion.Group},
					Resources: []string{"etcds/status", "etcds/finalizers", "etcdcopybackupstasks/status", "etcdcopybackupstasks/finalizers"},
					Verbs:     []string{"get", "update", "patch", "create"},
				},
				{
					APIGroups: []string{coordinationv1.GroupName},
					Resources: []string{"leases"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"},
				},
				{
					APIGroups: []string{corev1.GroupName},
					Resources: []string{"persistentvolumeclaims"},
					Verbs:     []string{"get", "list", "watch"},
				},
				{
					APIGroups: []string{policyv1beta1.GroupName},
					Resources: []string{"poddisruptionbudgets"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
			},
		}

		clusterRoleBinding = &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   druidRBACName,
				Labels: labels(),
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     druidRBACName,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      druidServiceAccountName,
					Namespace: b.namespace,
				},
			},
		}

		configMapImageVectorOverwrite = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidConfigMapImageVectorOverwriteNamePrefix,
				Namespace: b.namespace,
				Labels:    labels(),
			},
		}

		vpaUpdateMode = vpaautoscalingv1.UpdateModeAuto
		vpa           = &vpaautoscalingv1.VerticalPodAutoscaler{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidVPAName,
				Namespace: b.namespace,
				Labels:    labels(),
			},
			Spec: vpaautoscalingv1.VerticalPodAutoscalerSpec{
				TargetRef: &autoscalingv1.CrossVersionObjectReference{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       "Deployment",
					Name:       druidDeploymentName,
				},
				UpdatePolicy: &vpaautoscalingv1.PodUpdatePolicy{
					UpdateMode: &vpaUpdateMode,
				},
				ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
					ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{{
						ContainerName: vpaautoscalingv1.DefaultContainerResourcePolicy,
						MinAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("100M"),
						},
					}},
				},
			},
		}

		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      druidDeploymentName,
				Namespace: b.namespace,
				Labels:    labels(),
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             pointer.Int32(1),
				RevisionHistoryLimit: pointer.Int32(1),
				Selector: &metav1.LabelSelector{
					MatchLabels: labels(),
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels(),
					},
					Spec: corev1.PodSpec{
						PriorityClassName:  v1beta1constants.PriorityClassNameSeedSystem800,
						ServiceAccountName: druidServiceAccountName,
						Containers: []corev1.Container{
							{
								Name:            Druid,
								Image:           b.image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         getDruidDeployCommands(b.config),
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
								},
								Ports: []corev1.ContainerPort{{
									ContainerPort: 9569,
								}},
							},
						},
					},
				},
			},
		}

		resourcesToAdd = []client.Object{
			serviceAccount,
			clusterRole,
			clusterRoleBinding,
			vpa,
		}
	)

	if b.imageVectorOverwrite != nil {
		configMapImageVectorOverwrite.Data = map[string]string{druidConfigMapImageVectorOverwriteDataKey: *b.imageVectorOverwrite}
		utilruntime.Must(kutil.MakeUnique(configMapImageVectorOverwrite))
		resourcesToAdd = append(resourcesToAdd, configMapImageVectorOverwrite)

		deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: druidDeploymentVolumeNameImageVectorOverwrite,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapImageVectorOverwrite.Name,
					},
				},
			},
		})
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(deployment.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      druidDeploymentVolumeNameImageVectorOverwrite,
			MountPath: druidDeploymentVolumeMountPathImageVectorOverwrite,
			ReadOnly:  true,
		})
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  imagevector.OverrideEnv,
			Value: druidDeploymentVolumeMountPathImageVectorOverwrite + "/" + druidConfigMapImageVectorOverwriteDataKey,
		})

		utilruntime.Must(references.InjectAnnotations(deployment))
	}

	resources, err := registry.AddAllAndSerialize(append(resourcesToAdd, deployment)...)
	if err != nil {
		return err
	}
	resources["crd.yaml"] = []byte(CrdYAML)
	resources["crdEtcdCopyBackupsTask.yaml"] = []byte(etcdCopyBackupsTaskCRDYaml)

	return managedresources.CreateForSeed(ctx, b.client, b.namespace, managedResourceControlName, false, resources)
}

func getDruidDeployCommands(gardenletConf *config.GardenletConfiguration) []string {
	command := []string{"/etcd-druid"}
	command = append(command, "--enable-leader-election=true")
	command = append(command, "--ignore-operation-annotation=false")
	command = append(command, "--disable-etcd-serviceaccount-automount=true")

	if gardenletConf == nil {
		// TODO(abdasgupta): Following line to add 50 workers is only for backward compatibility. Please, remove.
		command = append(command, "--workers=50")
		return command
	}

	config := gardenletConf.ETCDConfig
	command = append(command, "--workers="+strconv.FormatInt(*config.ETCDController.Workers, 10))
	command = append(command, "--custodian-workers="+strconv.FormatInt(*config.CustodianController.Workers, 10))
	command = append(command, "--compaction-workers="+strconv.FormatInt(*config.BackupCompactionController.Workers, 10))
	command = append(command, "--enable-backup-compaction="+strconv.FormatBool(*config.BackupCompactionController.EnableBackupCompaction))
	command = append(command, "--etcd-events-threshold="+strconv.FormatInt(*config.BackupCompactionController.EventsThreshold, 10))
	if config.BackupCompactionController.ActiveDeadlineDuration != nil {
		command = append(command, "--active-deadline-duration="+config.BackupCompactionController.ActiveDeadlineDuration.Duration.String())
	}

	return command
}

func (b *bootstrapper) Destroy(ctx context.Context) error {
	etcdList := &druidv1alpha1.EtcdList{}
	// Need to check for both error types. The DynamicRestMapper can hold a stale cache returning a path to a non-existing api-resource leading to a NotFound error.
	if err := b.client.List(ctx, etcdList); err != nil && !meta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
		return err
	}

	if len(etcdList.Items) > 0 {
		return fmt.Errorf("cannot debootstrap etcd-druid because there are still druidv1alpha1.Etcd resources left in the cluster")
	}

	if err := gutil.ConfirmDeletion(ctx, b.client, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: crdName}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	etcdCopyBackupsTaskList := &druidv1alpha1.EtcdCopyBackupsTaskList{}
	if err := b.client.List(ctx, etcdCopyBackupsTaskList); err != nil && !meta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
		return err
	}

	if len(etcdCopyBackupsTaskList.Items) > 0 {
		return fmt.Errorf("cannot debootstrap etcd-druid because there are still druidv1alpha1.EtcdCopyBackupsTask resources left in the cluster")
	}

	if err := gutil.ConfirmDeletion(ctx, b.client, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: etcdCopyBackupsTaskCRDName}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	return managedresources.DeleteForSeed(ctx, b.client, b.namespace, managedResourceControlName)
}

// TimeoutWaitForManagedResource is the timeout used while waiting for the ManagedResources to become healthy
// or deleted.
var TimeoutWaitForManagedResource = 2 * time.Minute

func (b *bootstrapper) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilHealthy(timeoutCtx, b.client, b.namespace, managedResourceControlName)
}

func (b *bootstrapper) WaitCleanup(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForManagedResource)
	defer cancel()

	return managedresources.WaitUntilDeleted(timeoutCtx, b.client, b.namespace, managedResourceControlName)
}

const (
	crdName = "etcds.druid.gardener.cloud"

	// CrdYAML is yaml representation of the custom resource of the ETCD.
	CrdYAML = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ` + crdName + `
  annotations:
    controller-gen.kubebuilder.io/version: v0.7.0
  labels:
    ` + gutil.DeletionProtected + `: "true"
spec:
  group: druid.gardener.cloud
  names:
    kind: Etcd
    listKind: EtcdList
    plural: etcds
    singular: etcd
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - jsonPath: .status.ready
      name: Ready
      type: string
    - jsonPath: .metadata.creationTimestamp
      name: Age
      type: date
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: Etcd is the Schema for the etcds API
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: EtcdSpec defines the desired state of Etcd
            properties:
              annotations:
                additionalProperties:
                  type: string
                type: object
              backup:
                description: BackupSpec defines parameters associated with the full
                  and delta snapshots of etcd.
                properties:
                  compactionResources:
                    description: 'CompactionResources defines compute Resources required
                      by compaction job. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/'
                    properties:
                      limits:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Limits describes the maximum amount of compute
                          resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                      requests:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Requests describes the minimum amount of compute
                          resources required. If Requests is omitted for a container,
                          it defaults to Limits if that is explicitly specified, otherwise
                          to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                    type: object
                  compression:
                    description: SnapshotCompression defines the specification for
                      compression of Snapshots.
                    properties:
                      enabled:
                        type: boolean
                      policy:
                        description: CompressionPolicy defines the type of policy
                          for compression of snapshots.
                        enum:
                        - gzip
                        - lzw
                        - zlib
                        type: string
                    type: object
                  deltaSnapshotMemoryLimit:
                    anyOf:
                    - type: integer
                    - type: string
                    description: DeltaSnapshotMemoryLimit defines the memory limit
                      after which delta snapshots will be taken
                    pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                    x-kubernetes-int-or-string: true
                  deltaSnapshotPeriod:
                    description: DeltaSnapshotPeriod defines the period after which
                      delta snapshots will be taken
                    type: string
                  enableProfiling:
                    description: EnableProfiling defines if profiling should be enabled
                      for the etcd-backup-restore-sidecar
                    type: boolean
                  etcdSnapshotTimeout:
                    description: EtcdSnapshotTimeout defines the timeout duration
                      for etcd FullSnapshot operation
                    type: string
                  fullSnapshotSchedule:
                    description: FullSnapshotSchedule defines the cron standard schedule
                      for full snapshots.
                    type: string
                  garbageCollectionPeriod:
                    description: GarbageCollectionPeriod defines the period for garbage
                      collecting old backups
                    type: string
                  garbageCollectionPolicy:
                    description: GarbageCollectionPolicy defines the policy for garbage
                      collecting old backups
                    enum:
                    - Exponential
                    - LimitBased
                    type: string
                  image:
                    description: Image defines the etcd container image and tag
                    type: string
                  leaderElection:
                    description: LeaderElection defines parameters related to the
                      LeaderElection configuration.
                    properties:
                      etcdConnectionTimeout:
                        description: EtcdConnectionTimeout defines the timeout duration
                          for etcd client connection during leader election.
                        type: string
                      reelectionPeriod:
                        description: ReelectionPeriod defines the Period after which
                          leadership status of corresponding etcd is checked.
                        type: string
                    type: object
                  ownerCheck:
                    description: OwnerCheck defines parameters related to checking
                      if the cluster owner, as specified in the owner DNS record,
                      is the expected one.
                    properties:
                      dnsCacheTTL:
                        description: DNSCacheTTL is the DNS cache TTL for owner checks.
                        type: string
                      id:
                        description: ID is the owner id value that is expected to
                          be found in the owner DNS record.
                        type: string
                      interval:
                        description: Interval is the time interval between owner checks.
                        type: string
                      name:
                        description: Name is the domain name of the owner DNS record.
                        type: string
                      timeout:
                        description: Timeout is the timeout for owner checks.
                        type: string
                    required:
                    - id
                    - name
                    type: object
                  port:
                    description: Port define the port on which etcd-backup-restore
                      server will be exposed.
                    format: int32
                    type: integer
                  resources:
                    description: 'Resources defines compute Resources required by
                      backup-restore container. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/'
                    properties:
                      limits:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Limits describes the maximum amount of compute
                          resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                      requests:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Requests describes the minimum amount of compute
                          resources required. If Requests is omitted for a container,
                          it defaults to Limits if that is explicitly specified, otherwise
                          to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                    type: object
                  store:
                    description: Store defines the specification of object store provider
                      for storing backups.
                    properties:
                      container:
                        description: Container is the name of the container the backup
                          is stored at.
                        type: string
                      prefix:
                        description: Prefix is the prefix used for the store.
                        type: string
                      provider:
                        description: Provider is the name of the backup provider.
                        type: string
                      secretRef:
                        description: SecretRef is the reference to the secret which
                          used to connect to the backup store.
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                    required:
                    - prefix
                    type: object
                  tls:
                    description: TLSConfig hold the TLS configuration details.
                    properties:
                      clientTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      serverTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      tlsCASecretRef:
                        description: SecretReference defines a reference to a secret.
                        properties:
                          dataKey:
                            description: DataKey is the name of the key in the data
                              map containing the credentials.
                            type: string
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                    required:
                    - serverTLSSecretRef
                    - tlsCASecretRef
                    type: object
                type: object
              etcd:
                description: EtcdConfig defines parameters associated etcd deployed
                properties:
                  authSecretRef:
                    description: SecretReference represents a Secret Reference. It
                      has enough information to retrieve secret in any namespace
                    properties:
                      name:
                        description: Name is unique within a namespace to reference
                          a secret resource.
                        type: string
                      namespace:
                        description: Namespace defines the space within which the
                          secret name must be unique.
                        type: string
                    type: object
                  clientPort:
                    format: int32
                    type: integer
                  clientUrlTls:
                    description: ClientUrlTLS contains the ca, server TLS and client
                      TLS secrets for client communication to ETCD cluster
                    properties:
                      clientTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      serverTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      tlsCASecretRef:
                        description: SecretReference defines a reference to a secret.
                        properties:
                          dataKey:
                            description: DataKey is the name of the key in the data
                              map containing the credentials.
                            type: string
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                    required:
                    - serverTLSSecretRef
                    - tlsCASecretRef
                    type: object
                  defragmentationSchedule:
                    description: DefragmentationSchedule defines the cron standard
                      schedule for defragmentation of etcd.
                    type: string
                  etcdDefragTimeout:
                    description: EtcdDefragTimeout defines the timeout duration for
                      etcd defrag call
                    type: string
                  heartbeatDuration:
                    description: HeartbeatDuration defines the duration for members
                      to send heartbeats. The default value is 10s.
                    type: string
                  image:
                    description: Image defines the etcd container image and tag
                    type: string
                  metrics:
                    description: Metrics defines the level of detail for exported
                      metrics of etcd, specify 'extensive' to include histogram metrics.
                    enum:
                    - basic
                    - extensive
                    type: string
                  peerUrlTls:
                    description: PeerUrlTLS contains the ca and server TLS secrets
                      for peer communication within ETCD cluster Currently, PeerUrlTLS
                      does not require client TLS secrets for gardener implementation
                      of ETCD cluster.
                    properties:
                      clientTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      serverTLSSecretRef:
                        description: SecretReference represents a Secret Reference.
                          It has enough information to retrieve secret in any namespace
                        properties:
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                      tlsCASecretRef:
                        description: SecretReference defines a reference to a secret.
                        properties:
                          dataKey:
                            description: DataKey is the name of the key in the data
                              map containing the credentials.
                            type: string
                          name:
                            description: Name is unique within a namespace to reference
                              a secret resource.
                            type: string
                          namespace:
                            description: Namespace defines the space within which
                              the secret name must be unique.
                            type: string
                        type: object
                    required:
                    - serverTLSSecretRef
                    - tlsCASecretRef
                    type: object
                  quota:
                    anyOf:
                    - type: integer
                    - type: string
                    description: Quota defines the etcd DB quota.
                    pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                    x-kubernetes-int-or-string: true
                  resources:
                    description: 'Resources defines the compute Resources required
                      by etcd container. More info: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/'
                    properties:
                      limits:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Limits describes the maximum amount of compute
                          resources allowed. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                      requests:
                        additionalProperties:
                          anyOf:
                          - type: integer
                          - type: string
                          pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                          x-kubernetes-int-or-string: true
                        description: 'Requests describes the minimum amount of compute
                          resources required. If Requests is omitted for a container,
                          it defaults to Limits if that is explicitly specified, otherwise
                          to an implementation-defined value. More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/'
                        type: object
                    type: object
                  serverPort:
                    format: int32
                    type: integer
                type: object
              labels:
                additionalProperties:
                  type: string
                type: object
              priorityClassName:
                description: PriorityClassName is the name of a priority class that
                  shall be used for the etcd pods.
                type: string
              replicas:
                format: int32
                type: integer
              schedulingConstraints:
                description: SchedulingConstraints defines the different scheduling
                  constraints that must be applied to the pod spec in the etcd statefulset.
                  Currently supported constraints are Affinity and TopologySpreadConstraints.
                properties:
                  affinity:
                    description: Affinity defines the various affinity and anti-affinity
                      rules for a pod that are honoured by the kube-scheduler.
                    properties:
                      nodeAffinity:
                        description: Describes node affinity scheduling rules for
                          the pod.
                        properties:
                          preferredDuringSchedulingIgnoredDuringExecution:
                            description: The scheduler will prefer to schedule pods
                              to nodes that satisfy the affinity expressions specified
                              by this field, but it may choose a node that violates
                              one or more of the expressions. The node that is most
                              preferred is the one with the greatest sum of weights,
                              i.e. for each node that meets all of the scheduling
                              requirements (resource request, requiredDuringScheduling
                              affinity expressions, etc.), compute a sum by iterating
                              through the elements of this field and adding "weight"
                              to the sum if the node matches the corresponding matchExpressions;
                              the node(s) with the highest sum are the most preferred.
                            items:
                              description: An empty preferred scheduling term matches
                                all objects with implicit weight 0 (i.e. it's a no-op).
                                A null preferred scheduling term matches no objects
                                (i.e. is also a no-op).
                              properties:
                                preference:
                                  description: A node selector term, associated with
                                    the corresponding weight.
                                  properties:
                                    matchExpressions:
                                      description: A list of node selector requirements
                                        by node's labels.
                                      items:
                                        description: A node selector requirement is
                                          a selector that contains values, a key,
                                          and an operator that relates the key and
                                          values.
                                        properties:
                                          key:
                                            description: The label key that the selector
                                              applies to.
                                            type: string
                                          operator:
                                            description: Represents a key's relationship
                                              to a set of values. Valid operators
                                              are In, NotIn, Exists, DoesNotExist.
                                              Gt, and Lt.
                                            type: string
                                          values:
                                            description: An array of string values.
                                              If the operator is In or NotIn, the
                                              values array must be non-empty. If the
                                              operator is Exists or DoesNotExist,
                                              the values array must be empty. If the
                                              operator is Gt or Lt, the values array
                                              must have a single element, which will
                                              be interpreted as an integer. This array
                                              is replaced during a strategic merge
                                              patch.
                                            items:
                                              type: string
                                            type: array
                                        required:
                                        - key
                                        - operator
                                        type: object
                                      type: array
                                    matchFields:
                                      description: A list of node selector requirements
                                        by node's fields.
                                      items:
                                        description: A node selector requirement is
                                          a selector that contains values, a key,
                                          and an operator that relates the key and
                                          values.
                                        properties:
                                          key:
                                            description: The label key that the selector
                                              applies to.
                                            type: string
                                          operator:
                                            description: Represents a key's relationship
                                              to a set of values. Valid operators
                                              are In, NotIn, Exists, DoesNotExist.
                                              Gt, and Lt.
                                            type: string
                                          values:
                                            description: An array of string values.
                                              If the operator is In or NotIn, the
                                              values array must be non-empty. If the
                                              operator is Exists or DoesNotExist,
                                              the values array must be empty. If the
                                              operator is Gt or Lt, the values array
                                              must have a single element, which will
                                              be interpreted as an integer. This array
                                              is replaced during a strategic merge
                                              patch.
                                            items:
                                              type: string
                                            type: array
                                        required:
                                        - key
                                        - operator
                                        type: object
                                      type: array
                                  type: object
                                weight:
                                  description: Weight associated with matching the
                                    corresponding nodeSelectorTerm, in the range 1-100.
                                  format: int32
                                  type: integer
                              required:
                              - preference
                              - weight
                              type: object
                            type: array
                          requiredDuringSchedulingIgnoredDuringExecution:
                            description: If the affinity requirements specified by
                              this field are not met at scheduling time, the pod will
                              not be scheduled onto the node. If the affinity requirements
                              specified by this field cease to be met at some point
                              during pod execution (e.g. due to an update), the system
                              may or may not try to eventually evict the pod from
                              its node.
                            properties:
                              nodeSelectorTerms:
                                description: Required. A list of node selector terms.
                                  The terms are ORed.
                                items:
                                  description: A null or empty node selector term
                                    matches no objects. The requirements of them are
                                    ANDed. The TopologySelectorTerm type implements
                                    a subset of the NodeSelectorTerm.
                                  properties:
                                    matchExpressions:
                                      description: A list of node selector requirements
                                        by node's labels.
                                      items:
                                        description: A node selector requirement is
                                          a selector that contains values, a key,
                                          and an operator that relates the key and
                                          values.
                                        properties:
                                          key:
                                            description: The label key that the selector
                                              applies to.
                                            type: string
                                          operator:
                                            description: Represents a key's relationship
                                              to a set of values. Valid operators
                                              are In, NotIn, Exists, DoesNotExist.
                                              Gt, and Lt.
                                            type: string
                                          values:
                                            description: An array of string values.
                                              If the operator is In or NotIn, the
                                              values array must be non-empty. If the
                                              operator is Exists or DoesNotExist,
                                              the values array must be empty. If the
                                              operator is Gt or Lt, the values array
                                              must have a single element, which will
                                              be interpreted as an integer. This array
                                              is replaced during a strategic merge
                                              patch.
                                            items:
                                              type: string
                                            type: array
                                        required:
                                        - key
                                        - operator
                                        type: object
                                      type: array
                                    matchFields:
                                      description: A list of node selector requirements
                                        by node's fields.
                                      items:
                                        description: A node selector requirement is
                                          a selector that contains values, a key,
                                          and an operator that relates the key and
                                          values.
                                        properties:
                                          key:
                                            description: The label key that the selector
                                              applies to.
                                            type: string
                                          operator:
                                            description: Represents a key's relationship
                                              to a set of values. Valid operators
                                              are In, NotIn, Exists, DoesNotExist.
                                              Gt, and Lt.
                                            type: string
                                          values:
                                            description: An array of string values.
                                              If the operator is In or NotIn, the
                                              values array must be non-empty. If the
                                              operator is Exists or DoesNotExist,
                                              the values array must be empty. If the
                                              operator is Gt or Lt, the values array
                                              must have a single element, which will
                                              be interpreted as an integer. This array
                                              is replaced during a strategic merge
                                              patch.
                                            items:
                                              type: string
                                            type: array
                                        required:
                                        - key
                                        - operator
                                        type: object
                                      type: array
                                  type: object
                                type: array
                            required:
                            - nodeSelectorTerms
                            type: object
                        type: object
                      podAffinity:
                        description: Describes pod affinity scheduling rules (e.g.
                          co-locate this pod in the same node, zone, etc. as some
                          other pod(s)).
                        properties:
                          preferredDuringSchedulingIgnoredDuringExecution:
                            description: The scheduler will prefer to schedule pods
                              to nodes that satisfy the affinity expressions specified
                              by this field, but it may choose a node that violates
                              one or more of the expressions. The node that is most
                              preferred is the one with the greatest sum of weights,
                              i.e. for each node that meets all of the scheduling
                              requirements (resource request, requiredDuringScheduling
                              affinity expressions, etc.), compute a sum by iterating
                              through the elements of this field and adding "weight"
                              to the sum if the node has pods which matches the corresponding
                              podAffinityTerm; the node(s) with the highest sum are
                              the most preferred.
                            items:
                              description: The weights of all of the matched WeightedPodAffinityTerm
                                fields are added per-node to find the most preferred
                                node(s)
                              properties:
                                podAffinityTerm:
                                  description: Required. A pod affinity term, associated
                                    with the corresponding weight.
                                  properties:
                                    labelSelector:
                                      description: A label query over a set of resources,
                                        in this case pods.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: A label selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: operator represents a
                                                  key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists
                                                  and DoesNotExist.
                                                type: string
                                              values:
                                                description: values is an array of
                                                  string values. If the operator is
                                                  In or NotIn, the values array must
                                                  be non-empty. If the operator is
                                                  Exists or DoesNotExist, the values
                                                  array must be empty. This array
                                                  is replaced during a strategic merge
                                                  patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: matchLabels is a map of {key,value}
                                            pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions,
                                            whose key field is "key", the operator
                                            is "In", and the values array contains
                                            only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                    namespaceSelector:
                                      description: A label query over the set of namespaces
                                        that the term applies to. The term is applied
                                        to the union of the namespaces selected by
                                        this field and the ones listed in the namespaces
                                        field. null selector and null or empty namespaces
                                        list means "this pod's namespace". An empty
                                        selector ({}) matches all namespaces. This
                                        field is beta-level and is only honored when
                                        PodAffinityNamespaceSelector feature is enabled.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: A label selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: operator represents a
                                                  key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists
                                                  and DoesNotExist.
                                                type: string
                                              values:
                                                description: values is an array of
                                                  string values. If the operator is
                                                  In or NotIn, the values array must
                                                  be non-empty. If the operator is
                                                  Exists or DoesNotExist, the values
                                                  array must be empty. This array
                                                  is replaced during a strategic merge
                                                  patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: matchLabels is a map of {key,value}
                                            pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions,
                                            whose key field is "key", the operator
                                            is "In", and the values array contains
                                            only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                    namespaces:
                                      description: namespaces specifies a static list
                                        of namespace names that the term applies to.
                                        The term is applied to the union of the namespaces
                                        listed in this field and the ones selected
                                        by namespaceSelector. null or empty namespaces
                                        list and null namespaceSelector means "this
                                        pod's namespace"
                                      items:
                                        type: string
                                      type: array
                                    topologyKey:
                                      description: This pod should be co-located (affinity)
                                        or not co-located (anti-affinity) with the
                                        pods matching the labelSelector in the specified
                                        namespaces, where co-located is defined as
                                        running on a node whose value of the label
                                        with key topologyKey matches that of any node
                                        on which any of the selected pods is running.
                                        Empty topologyKey is not allowed.
                                      type: string
                                  required:
                                  - topologyKey
                                  type: object
                                weight:
                                  description: weight associated with matching the
                                    corresponding podAffinityTerm, in the range 1-100.
                                  format: int32
                                  type: integer
                              required:
                              - podAffinityTerm
                              - weight
                              type: object
                            type: array
                          requiredDuringSchedulingIgnoredDuringExecution:
                            description: If the affinity requirements specified by
                              this field are not met at scheduling time, the pod will
                              not be scheduled onto the node. If the affinity requirements
                              specified by this field cease to be met at some point
                              during pod execution (e.g. due to a pod label update),
                              the system may or may not try to eventually evict the
                              pod from its node. When there are multiple elements,
                              the lists of nodes corresponding to each podAffinityTerm
                              are intersected, i.e. all terms must be satisfied.
                            items:
                              description: Defines a set of pods (namely those matching
                                the labelSelector relative to the given namespace(s))
                                that this pod should be co-located (affinity) or not
                                co-located (anti-affinity) with, where co-located
                                is defined as running on a node whose value of the
                                label with key <topologyKey> matches that of any node
                                on which a pod of the set of pods is running
                              properties:
                                labelSelector:
                                  description: A label query over a set of resources,
                                    in this case pods.
                                  properties:
                                    matchExpressions:
                                      description: matchExpressions is a list of label
                                        selector requirements. The requirements are
                                        ANDed.
                                      items:
                                        description: A label selector requirement
                                          is a selector that contains values, a key,
                                          and an operator that relates the key and
                                          values.
                                        properties:
                                          key:
                                            description: key is the label key that
                                              the selector applies to.
                                            type: string
                                          operator:
                                            description: operator represents a key's
                                              relationship to a set of values. Valid
                                              operators are In, NotIn, Exists and
                                              DoesNotExist.
                                            type: string
                                          values:
                                            description: values is an array of string
                                              values. If the operator is In or NotIn,
                                              the values array must be non-empty.
                                              If the operator is Exists or DoesNotExist,
                                              the values array must be empty. This
                                              array is replaced during a strategic
                                              merge patch.
                                            items:
                                              type: string
                                            type: array
                                        required:
                                        - key
                                        - operator
                                        type: object
                                      type: array
                                    matchLabels:
                                      additionalProperties:
                                        type: string
                                      description: matchLabels is a map of {key,value}
                                        pairs. A single {key,value} in the matchLabels
                                        map is equivalent to an element of matchExpressions,
                                        whose key field is "key", the operator is
                                        "In", and the values array contains only "value".
                                        The requirements are ANDed.
                                      type: object
                                  type: object
                                namespaceSelector:
                                  description: A label query over the set of namespaces
                                    that the term applies to. The term is applied
                                    to the union of the namespaces selected by this
                                    field and the ones listed in the namespaces field.
                                    null selector and null or empty namespaces list
                                    means "this pod's namespace". An empty selector
                                    ({}) matches all namespaces. This field is beta-level
                                    and is only honored when PodAffinityNamespaceSelector
                                    feature is enabled.
                                  properties:
                                    matchExpressions:
                                      description: matchExpressions is a list of label
                                        selector requirements. The requirements are
                                        ANDed.
                                      items:
                                        description: A label selector requirement
                                          is a selector that contains values, a key,
                                          and an operator that relates the key and
                                          values.
                                        properties:
                                          key:
                                            description: key is the label key that
                                              the selector applies to.
                                            type: string
                                          operator:
                                            description: operator represents a key's
                                              relationship to a set of values. Valid
                                              operators are In, NotIn, Exists and
                                              DoesNotExist.
                                            type: string
                                          values:
                                            description: values is an array of string
                                              values. If the operator is In or NotIn,
                                              the values array must be non-empty.
                                              If the operator is Exists or DoesNotExist,
                                              the values array must be empty. This
                                              array is replaced during a strategic
                                              merge patch.
                                            items:
                                              type: string
                                            type: array
                                        required:
                                        - key
                                        - operator
                                        type: object
                                      type: array
                                    matchLabels:
                                      additionalProperties:
                                        type: string
                                      description: matchLabels is a map of {key,value}
                                        pairs. A single {key,value} in the matchLabels
                                        map is equivalent to an element of matchExpressions,
                                        whose key field is "key", the operator is
                                        "In", and the values array contains only "value".
                                        The requirements are ANDed.
                                      type: object
                                  type: object
                                namespaces:
                                  description: namespaces specifies a static list
                                    of namespace names that the term applies to. The
                                    term is applied to the union of the namespaces
                                    listed in this field and the ones selected by
                                    namespaceSelector. null or empty namespaces list
                                    and null namespaceSelector means "this pod's namespace"
                                  items:
                                    type: string
                                  type: array
                                topologyKey:
                                  description: This pod should be co-located (affinity)
                                    or not co-located (anti-affinity) with the pods
                                    matching the labelSelector in the specified namespaces,
                                    where co-located is defined as running on a node
                                    whose value of the label with key topologyKey
                                    matches that of any node on which any of the selected
                                    pods is running. Empty topologyKey is not allowed.
                                  type: string
                              required:
                              - topologyKey
                              type: object
                            type: array
                        type: object
                      podAntiAffinity:
                        description: Describes pod anti-affinity scheduling rules
                          (e.g. avoid putting this pod in the same node, zone, etc.
                          as some other pod(s)).
                        properties:
                          preferredDuringSchedulingIgnoredDuringExecution:
                            description: The scheduler will prefer to schedule pods
                              to nodes that satisfy the anti-affinity expressions
                              specified by this field, but it may choose a node that
                              violates one or more of the expressions. The node that
                              is most preferred is the one with the greatest sum of
                              weights, i.e. for each node that meets all of the scheduling
                              requirements (resource request, requiredDuringScheduling
                              anti-affinity expressions, etc.), compute a sum by iterating
                              through the elements of this field and adding "weight"
                              to the sum if the node has pods which matches the corresponding
                              podAffinityTerm; the node(s) with the highest sum are
                              the most preferred.
                            items:
                              description: The weights of all of the matched WeightedPodAffinityTerm
                                fields are added per-node to find the most preferred
                                node(s)
                              properties:
                                podAffinityTerm:
                                  description: Required. A pod affinity term, associated
                                    with the corresponding weight.
                                  properties:
                                    labelSelector:
                                      description: A label query over a set of resources,
                                        in this case pods.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: A label selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: operator represents a
                                                  key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists
                                                  and DoesNotExist.
                                                type: string
                                              values:
                                                description: values is an array of
                                                  string values. If the operator is
                                                  In or NotIn, the values array must
                                                  be non-empty. If the operator is
                                                  Exists or DoesNotExist, the values
                                                  array must be empty. This array
                                                  is replaced during a strategic merge
                                                  patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: matchLabels is a map of {key,value}
                                            pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions,
                                            whose key field is "key", the operator
                                            is "In", and the values array contains
                                            only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                    namespaceSelector:
                                      description: A label query over the set of namespaces
                                        that the term applies to. The term is applied
                                        to the union of the namespaces selected by
                                        this field and the ones listed in the namespaces
                                        field. null selector and null or empty namespaces
                                        list means "this pod's namespace". An empty
                                        selector ({}) matches all namespaces. This
                                        field is beta-level and is only honored when
                                        PodAffinityNamespaceSelector feature is enabled.
                                      properties:
                                        matchExpressions:
                                          description: matchExpressions is a list
                                            of label selector requirements. The requirements
                                            are ANDed.
                                          items:
                                            description: A label selector requirement
                                              is a selector that contains values,
                                              a key, and an operator that relates
                                              the key and values.
                                            properties:
                                              key:
                                                description: key is the label key
                                                  that the selector applies to.
                                                type: string
                                              operator:
                                                description: operator represents a
                                                  key's relationship to a set of values.
                                                  Valid operators are In, NotIn, Exists
                                                  and DoesNotExist.
                                                type: string
                                              values:
                                                description: values is an array of
                                                  string values. If the operator is
                                                  In or NotIn, the values array must
                                                  be non-empty. If the operator is
                                                  Exists or DoesNotExist, the values
                                                  array must be empty. This array
                                                  is replaced during a strategic merge
                                                  patch.
                                                items:
                                                  type: string
                                                type: array
                                            required:
                                            - key
                                            - operator
                                            type: object
                                          type: array
                                        matchLabels:
                                          additionalProperties:
                                            type: string
                                          description: matchLabels is a map of {key,value}
                                            pairs. A single {key,value} in the matchLabels
                                            map is equivalent to an element of matchExpressions,
                                            whose key field is "key", the operator
                                            is "In", and the values array contains
                                            only "value". The requirements are ANDed.
                                          type: object
                                      type: object
                                    namespaces:
                                      description: namespaces specifies a static list
                                        of namespace names that the term applies to.
                                        The term is applied to the union of the namespaces
                                        listed in this field and the ones selected
                                        by namespaceSelector. null or empty namespaces
                                        list and null namespaceSelector means "this
                                        pod's namespace"
                                      items:
                                        type: string
                                      type: array
                                    topologyKey:
                                      description: This pod should be co-located (affinity)
                                        or not co-located (anti-affinity) with the
                                        pods matching the labelSelector in the specified
                                        namespaces, where co-located is defined as
                                        running on a node whose value of the label
                                        with key topologyKey matches that of any node
                                        on which any of the selected pods is running.
                                        Empty topologyKey is not allowed.
                                      type: string
                                  required:
                                  - topologyKey
                                  type: object
                                weight:
                                  description: weight associated with matching the
                                    corresponding podAffinityTerm, in the range 1-100.
                                  format: int32
                                  type: integer
                              required:
                              - podAffinityTerm
                              - weight
                              type: object
                            type: array
                          requiredDuringSchedulingIgnoredDuringExecution:
                            description: If the anti-affinity requirements specified
                              by this field are not met at scheduling time, the pod
                              will not be scheduled onto the node. If the anti-affinity
                              requirements specified by this field cease to be met
                              at some point during pod execution (e.g. due to a pod
                              label update), the system may or may not try to eventually
                              evict the pod from its node. When there are multiple
                              elements, the lists of nodes corresponding to each podAffinityTerm
                              are intersected, i.e. all terms must be satisfied.
                            items:
                              description: Defines a set of pods (namely those matching
                                the labelSelector relative to the given namespace(s))
                                that this pod should be co-located (affinity) or not
                                co-located (anti-affinity) with, where co-located
                                is defined as running on a node whose value of the
                                label with key <topologyKey> matches that of any node
                                on which a pod of the set of pods is running
                              properties:
                                labelSelector:
                                  description: A label query over a set of resources,
                                    in this case pods.
                                  properties:
                                    matchExpressions:
                                      description: matchExpressions is a list of label
                                        selector requirements. The requirements are
                                        ANDed.
                                      items:
                                        description: A label selector requirement
                                          is a selector that contains values, a key,
                                          and an operator that relates the key and
                                          values.
                                        properties:
                                          key:
                                            description: key is the label key that
                                              the selector applies to.
                                            type: string
                                          operator:
                                            description: operator represents a key's
                                              relationship to a set of values. Valid
                                              operators are In, NotIn, Exists and
                                              DoesNotExist.
                                            type: string
                                          values:
                                            description: values is an array of string
                                              values. If the operator is In or NotIn,
                                              the values array must be non-empty.
                                              If the operator is Exists or DoesNotExist,
                                              the values array must be empty. This
                                              array is replaced during a strategic
                                              merge patch.
                                            items:
                                              type: string
                                            type: array
                                        required:
                                        - key
                                        - operator
                                        type: object
                                      type: array
                                    matchLabels:
                                      additionalProperties:
                                        type: string
                                      description: matchLabels is a map of {key,value}
                                        pairs. A single {key,value} in the matchLabels
                                        map is equivalent to an element of matchExpressions,
                                        whose key field is "key", the operator is
                                        "In", and the values array contains only "value".
                                        The requirements are ANDed.
                                      type: object
                                  type: object
                                namespaceSelector:
                                  description: A label query over the set of namespaces
                                    that the term applies to. The term is applied
                                    to the union of the namespaces selected by this
                                    field and the ones listed in the namespaces field.
                                    null selector and null or empty namespaces list
                                    means "this pod's namespace". An empty selector
                                    ({}) matches all namespaces. This field is beta-level
                                    and is only honored when PodAffinityNamespaceSelector
                                    feature is enabled.
                                  properties:
                                    matchExpressions:
                                      description: matchExpressions is a list of label
                                        selector requirements. The requirements are
                                        ANDed.
                                      items:
                                        description: A label selector requirement
                                          is a selector that contains values, a key,
                                          and an operator that relates the key and
                                          values.
                                        properties:
                                          key:
                                            description: key is the label key that
                                              the selector applies to.
                                            type: string
                                          operator:
                                            description: operator represents a key's
                                              relationship to a set of values. Valid
                                              operators are In, NotIn, Exists and
                                              DoesNotExist.
                                            type: string
                                          values:
                                            description: values is an array of string
                                              values. If the operator is In or NotIn,
                                              the values array must be non-empty.
                                              If the operator is Exists or DoesNotExist,
                                              the values array must be empty. This
                                              array is replaced during a strategic
                                              merge patch.
                                            items:
                                              type: string
                                            type: array
                                        required:
                                        - key
                                        - operator
                                        type: object
                                      type: array
                                    matchLabels:
                                      additionalProperties:
                                        type: string
                                      description: matchLabels is a map of {key,value}
                                        pairs. A single {key,value} in the matchLabels
                                        map is equivalent to an element of matchExpressions,
                                        whose key field is "key", the operator is
                                        "In", and the values array contains only "value".
                                        The requirements are ANDed.
                                      type: object
                                  type: object
                                namespaces:
                                  description: namespaces specifies a static list
                                    of namespace names that the term applies to. The
                                    term is applied to the union of the namespaces
                                    listed in this field and the ones selected by
                                    namespaceSelector. null or empty namespaces list
                                    and null namespaceSelector means "this pod's namespace"
                                  items:
                                    type: string
                                  type: array
                                topologyKey:
                                  description: This pod should be co-located (affinity)
                                    or not co-located (anti-affinity) with the pods
                                    matching the labelSelector in the specified namespaces,
                                    where co-located is defined as running on a node
                                    whose value of the label with key topologyKey
                                    matches that of any node on which any of the selected
                                    pods is running. Empty topologyKey is not allowed.
                                  type: string
                              required:
                              - topologyKey
                              type: object
                            type: array
                        type: object
                    type: object
                  topologySpreadConstraints:
                    description: TopologySpreadConstraints describes how a group of
                      pods ought to spread across topology domains, that are honoured
                      by the kube-scheduler.
                    items:
                      description: TopologySpreadConstraint specifies how to spread
                        matching pods among the given topology.
                      properties:
                        labelSelector:
                          description: LabelSelector is used to find matching pods.
                            Pods that match this label selector are counted to determine
                            the number of pods in their corresponding topology domain.
                          properties:
                            matchExpressions:
                              description: matchExpressions is a list of label selector
                                requirements. The requirements are ANDed.
                              items:
                                description: A label selector requirement is a selector
                                  that contains values, a key, and an operator that
                                  relates the key and values.
                                properties:
                                  key:
                                    description: key is the label key that the selector
                                      applies to.
                                    type: string
                                  operator:
                                    description: operator represents a key's relationship
                                      to a set of values. Valid operators are In,
                                      NotIn, Exists and DoesNotExist.
                                    type: string
                                  values:
                                    description: values is an array of string values.
                                      If the operator is In or NotIn, the values array
                                      must be non-empty. If the operator is Exists
                                      or DoesNotExist, the values array must be empty.
                                      This array is replaced during a strategic merge
                                      patch.
                                    items:
                                      type: string
                                    type: array
                                required:
                                - key
                                - operator
                                type: object
                              type: array
                            matchLabels:
                              additionalProperties:
                                type: string
                              description: matchLabels is a map of {key,value} pairs.
                                A single {key,value} in the matchLabels map is equivalent
                                to an element of matchExpressions, whose key field
                                is "key", the operator is "In", and the values array
                                contains only "value". The requirements are ANDed.
                              type: object
                          type: object
                        maxSkew:
                          description: 'MaxSkew describes the degree to which pods
                            may be unevenly distributed. When ` + "`whenUnsatisfiable=DoNotSchedule`" + `,
                            it is the maximum permitted difference between the number
                            of matching pods in the target topology and the global
                            minimum. For example, in a 3-zone cluster, MaxSkew is
                            set to 1, and pods with the same labelSelector spread
                            as 1/1/0: | zone1 | zone2 | zone3 | |   P   |   P   |       |
                            - if MaxSkew is 1, incoming pod can only be scheduled
                            to zone3 to become 1/1/1; scheduling it onto zone1(zone2)
                            would make the ActualSkew(2-0) on zone1(zone2) violate
                            MaxSkew(1). - if MaxSkew is 2, incoming pod can be scheduled
                            onto any zone. When ` + "`whenUnsatisfiable=ScheduleAnyway`" + `,
                            it is used to give higher precedence to topologies that
                            satisfy it. It''s a required field. Default value is 1
                            and 0 is not allowed.'
                          format: int32
                          type: integer
                        topologyKey:
                          description: TopologyKey is the key of node labels. Nodes
                            that have a label with this key and identical values are
                            considered to be in the same topology. We consider each
                            <key, value> as a "bucket", and try to put balanced number
                            of pods into each bucket. It's a required field.
                          type: string
                        whenUnsatisfiable:
                          description: 'WhenUnsatisfiable indicates how to deal with
                            a pod if it doesn''t satisfy the spread constraint. -
                            DoNotSchedule (default) tells the scheduler not to schedule
                            it. - ScheduleAnyway tells the scheduler to schedule the
                            pod in any location,   but giving higher precedence to
                            topologies that would help reduce the   skew. A constraint
                            is considered "Unsatisfiable" for an incoming pod if and
                            only if every possible node assigment for that pod would
                            violate "MaxSkew" on some topology. For example, in a
                            3-zone cluster, MaxSkew is set to 1, and pods with the
                            same labelSelector spread as 3/1/1: | zone1 | zone2 |
                            zone3 | | P P P |   P   |   P   | If WhenUnsatisfiable
                            is set to DoNotSchedule, incoming pod can only be scheduled
                            to zone2(zone3) to become 3/2/1(3/1/2) as ActualSkew(2-1)
                            on zone2(zone3) satisfies MaxSkew(1). In other words,
                            the cluster can still be imbalanced, but scheduler won''t
                            make it *more* imbalanced. It''s a required field.'
                          type: string
                      required:
                      - maxSkew
                      - topologyKey
                      - whenUnsatisfiable
                      type: object
                    type: array
                type: object
              selector:
                description: 'selector is a label query over pods that should match
                  the replica count. It must match the pod template''s labels. More
                  info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors'
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: A label selector requirement is a selector that
                        contains values, a key, and an operator that relates the key
                        and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: operator represents a key's relationship to
                            a set of values. Valid operators are In, NotIn, Exists
                            and DoesNotExist.
                          type: string
                        values:
                          description: values is an array of string values. If the
                            operator is In or NotIn, the values array must be non-empty.
                            If the operator is Exists or DoesNotExist, the values
                            array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: matchLabels is a map of {key,value} pairs. A single
                      {key,value} in the matchLabels map is equivalent to an element
                      of matchExpressions, whose key field is "key", the operator
                      is "In", and the values array contains only "value". The requirements
                      are ANDed.
                    type: object
                type: object
              sharedConfig:
                description: SharedConfig defines parameters shared and used by Etcd
                  as well as backup-restore sidecar.
                properties:
                  autoCompactionMode:
                    description: AutoCompactionMode defines the auto-compaction-mode:'periodic'
                      mode or 'revision' mode for etcd and embedded-Etcd of backup-restore
                      sidecar.
                    enum:
                    - periodic
                    - revision
                    type: string
                  autoCompactionRetention:
                    description: AutoCompactionRetention defines the auto-compaction-retention
                      length for etcd as well as for embedded-Etcd of backup-restore
                      sidecar.
                    type: string
                type: object
              storageCapacity:
                anyOf:
                - type: integer
                - type: string
                description: StorageCapacity defines the size of persistent volume.
                pattern: ^(\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))(([KMGTPE]i)|[numkMGTPE]|([eE](\+|-)?(([0-9]+(\.[0-9]*)?)|(\.[0-9]+))))?$
                x-kubernetes-int-or-string: true
              storageClass:
                description: 'StorageClass defines the name of the StorageClass required
                  by the claim. More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1'
                type: string
              volumeClaimTemplate:
                description: VolumeClaimTemplate defines the volume claim template
                  to be created
                type: string
            required:
            - backup
            - etcd
            - labels
            - replicas
            - selector
            type: object
          status:
            description: EtcdStatus defines the observed state of Etcd.
            properties:
              clusterSize:
                description: Cluster size is the size of the etcd cluster.
                format: int32
                type: integer
              conditions:
                description: Conditions represents the latest available observations
                  of an etcd's current state.
                items:
                  description: Condition holds the information about the state of
                    a resource.
                  properties:
                    lastTransitionTime:
                      description: Last time the condition transitioned from one status
                        to another.
                      format: date-time
                      type: string
                    lastUpdateTime:
                      description: Last time the condition was updated.
                      format: date-time
                      type: string
                    message:
                      description: A human readable message indicating details about
                        the transition.
                      type: string
                    reason:
                      description: The reason for the condition's last transition.
                      type: string
                    status:
                      description: Status of the condition, one of True, False, Unknown.
                      type: string
                    type:
                      description: Type of the Etcd condition.
                      type: string
                  required:
                  - lastTransitionTime
                  - lastUpdateTime
                  - message
                  - reason
                  - status
                  - type
                  type: object
                type: array
              currentReplicas:
                description: CurrentReplicas is the current replica count for the
                  etcd cluster.
                format: int32
                type: integer
              etcd:
                description: CrossVersionObjectReference contains enough information
                  to let you identify the referred resource.
                properties:
                  apiVersion:
                    description: API version of the referent
                    type: string
                  kind:
                    description: Kind of the referent
                    type: string
                  name:
                    description: Name of the referent
                    type: string
                type: object
              labelSelector:
                description: LabelSelector is a label query over pods that should
                  match the replica count. It must match the pod template's labels.
                properties:
                  matchExpressions:
                    description: matchExpressions is a list of label selector requirements.
                      The requirements are ANDed.
                    items:
                      description: A label selector requirement is a selector that
                        contains values, a key, and an operator that relates the key
                        and values.
                      properties:
                        key:
                          description: key is the label key that the selector applies
                            to.
                          type: string
                        operator:
                          description: operator represents a key's relationship to
                            a set of values. Valid operators are In, NotIn, Exists
                            and DoesNotExist.
                          type: string
                        values:
                          description: values is an array of string values. If the
                            operator is In or NotIn, the values array must be non-empty.
                            If the operator is Exists or DoesNotExist, the values
                            array must be empty. This array is replaced during a strategic
                            merge patch.
                          items:
                            type: string
                          type: array
                      required:
                      - key
                      - operator
                      type: object
                    type: array
                  matchLabels:
                    additionalProperties:
                      type: string
                    description: matchLabels is a map of {key,value} pairs. A single
                      {key,value} in the matchLabels map is equivalent to an element
                      of matchExpressions, whose key field is "key", the operator
                      is "In", and the values array contains only "value". The requirements
                      are ANDed.
                    type: object
                type: object
              lastError:
                description: LastError represents the last occurred error.
                type: string
              members:
                description: Members represents the members of the etcd cluster
                items:
                  description: EtcdMemberStatus holds information about a etcd cluster
                    membership.
                  properties:
                    id:
                      description: ID is the ID of the etcd member.
                      type: string
                    lastTransitionTime:
                      description: LastTransitionTime is the last time the condition's
                        status changed.
                      format: date-time
                      type: string
                    name:
                      description: Name is the name of the etcd member. It is the
                        name of the backing ` + "`Pod`" + `.
                      type: string
                    reason:
                      description: The reason for the condition's last transition.
                      type: string
                    role:
                      description: Role is the role in the etcd cluster, either ` + "`Leader`" + `
                        or ` + "`Member`" + `.
                      type: string
                    status:
                      description: Status of the condition, one of True, False, Unknown.
                      type: string
                  required:
                  - lastTransitionTime
                  - name
                  - reason
                  - status
                  type: object
                type: array
              observedGeneration:
                description: ObservedGeneration is the most recent generation observed
                  for this resource.
                format: int64
                type: integer
              ready:
                description: Ready represents the readiness of the etcd resource.
                type: boolean
              readyReplicas:
                description: ReadyReplicas is the count of replicas being ready in
                  the etcd cluster.
                format: int32
                type: integer
              replicas:
                description: Replicas is the replica count of the etcd resource.
                format: int32
                type: integer
              serviceName:
                description: ServiceName is the name of the etcd service.
                type: string
              updatedReplicas:
                description: UpdatedReplicas is the count of updated replicas in the
                  etcd cluster.
                format: int32
                type: integer
            type: object
        type: object
    served: true
    storage: true
    subresources:
      scale:
        labelSelectorPath: .status.labelSelector
        specReplicasPath: .spec.replicas
        statusReplicasPath: .status.replicas
      status: {}
`
	etcdCopyBackupsTaskCRDName = "etcdcopybackupstasks.druid.gardener.cloud"
	etcdCopyBackupsTaskCRDYaml = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.7.0
  creationTimestamp: null
  name: ` + etcdCopyBackupsTaskCRDName + `
  labels:
    ` + gutil.DeletionProtected + `: "true"
spec:
  group: druid.gardener.cloud
  names:
    kind: EtcdCopyBackupsTask
    listKind: EtcdCopyBackupsTaskList
    plural: etcdcopybackupstasks
    singular: etcdcopybackupstask
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - jsonPath: .metadata.creationTimestamp
      name: Age
      type: date
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: EtcdCopyBackupsTask is a task for copying etcd backups from a
          source to a target store.
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: EtcdCopyBackupsTaskSpec defines the parameters for the copy
              backups task.
            properties:
              maxBackupAge:
                description: MaxBackupAge is the maximum age in days that a backup
                  must have in order to be copied. By default all backups will be
                  copied.
                format: int32
                type: integer
              maxBackups:
                description: MaxBackups is the maximum number of backups that will
                  be copied starting with the most recent ones.
                format: int32
                type: integer
              sourceStore:
                description: SourceStore defines the specification of the source object
                  store provider for storing backups.
                properties:
                  container:
                    description: Container is the name of the container the backup
                      is stored at.
                    type: string
                  prefix:
                    description: Prefix is the prefix used for the store.
                    type: string
                  provider:
                    description: Provider is the name of the backup provider.
                    type: string
                  secretRef:
                    description: SecretRef is the reference to the secret which used
                      to connect to the backup store.
                    properties:
                      name:
                        description: Name is unique within a namespace to reference
                          a secret resource.
                        type: string
                      namespace:
                        description: Namespace defines the space within which the
                          secret name must be unique.
                        type: string
                    type: object
                required:
                - prefix
                type: object
              targetStore:
                description: TargetStore defines the specification of the target object
                  store provider for storing backups.
                properties:
                  container:
                    description: Container is the name of the container the backup
                      is stored at.
                    type: string
                  prefix:
                    description: Prefix is the prefix used for the store.
                    type: string
                  provider:
                    description: Provider is the name of the backup provider.
                    type: string
                  secretRef:
                    description: SecretRef is the reference to the secret which used
                      to connect to the backup store.
                    properties:
                      name:
                        description: Name is unique within a namespace to reference
                          a secret resource.
                        type: string
                      namespace:
                        description: Namespace defines the space within which the
                          secret name must be unique.
                        type: string
                    type: object
                required:
                - prefix
                type: object
              waitForFinalSnapshot:
                description: WaitForFinalSnapshot defines the parameters for waiting
                  for a final full snapshot before copying backups.
                properties:
                  enabled:
                    description: Enabled specifies whether to wait for a final full
                      snapshot before copying backups.
                    type: boolean
                  timeout:
                    description: Timeout is the timeout for waiting for a final full
                      snapshot. When this timeout expires, the copying of backups
                      will be performed anyway. No timeout or 0 means wait forever.
                    type: string
                required:
                - enabled
                type: object
            required:
            - sourceStore
            - targetStore
            type: object
          status:
            description: EtcdCopyBackupsTaskStatus defines the observed state of the
              copy backups task.
            properties:
              conditions:
                description: Conditions represents the latest available observations
                  of an object's current state.
                items:
                  description: Condition holds the information about the state of
                    a resource.
                  properties:
                    lastTransitionTime:
                      description: Last time the condition transitioned from one status
                        to another.
                      format: date-time
                      type: string
                    lastUpdateTime:
                      description: Last time the condition was updated.
                      format: date-time
                      type: string
                    message:
                      description: A human readable message indicating details about
                        the transition.
                      type: string
                    reason:
                      description: The reason for the condition's last transition.
                      type: string
                    status:
                      description: Status of the condition, one of True, False, Unknown.
                      type: string
                    type:
                      description: Type of the Etcd condition.
                      type: string
                  required:
                  - lastTransitionTime
                  - lastUpdateTime
                  - message
                  - reason
                  - status
                  - type
                  type: object
                type: array
              lastError:
                description: LastError represents the last occurred error.
                type: string
              observedGeneration:
                description: ObservedGeneration is the most recent generation observed
                  for this resource.
                format: int64
                type: integer
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
`
)
