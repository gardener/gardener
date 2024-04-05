// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	dwd "github.com/gardener/gardener/pkg/component/nodemanagement/dependencywatchdog"
	"github.com/gardener/gardener/pkg/controllerutils"
	resourcemanagerv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, _ cluster.Cluster) error {
	log.Info("Cleaning up GRM secret finalizers")
	if err := cleanupGRMSecretFinalizers(ctx, g.mgr.GetClient(), log); err != nil {
		return fmt.Errorf("failed to clean up GRM secret finalizers: %w", err)
	}

	log.Info("Updating shoot Prometheus config for connection to cache Prometheus and seed Alertmanager")
	if err := updateShootPrometheusConfigForConnectionToCachePrometheusAndSeedAlertManager(ctx, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Creating new secret and managed resource required by dependency-watchdog")
	if err := g.createNewDWDResources(ctx, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Cleaning up legacy 'shoot-core' ManagedResource")
	if err := cleanupShootCoreManagedResource(ctx, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Reconciling labels for PVC migrations")
	if err := reconcileLabelsForPVCMigrations(ctx, log, g.mgr.GetClient()); err != nil {
		return err
	}

	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient(), g.mgr.GetConfig()); err != nil {
		return err
	}

	return nil
}

// TODO(aaronfern): Remove this code after v1.93 has been released.
func (g *garden) createNewDWDResources(ctx context.Context, seedClient client.Client) error {
	namespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, namespaceList, client.MatchingLabels(map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot})); err != nil {
		return err
	}

	var tasks []flow.TaskFn
	for _, ns := range namespaceList.Items {
		if ns.DeletionTimestamp != nil || ns.Status.Phase == corev1.NamespaceTerminating {
			continue
		}
		namespace := ns
		tasks = append(tasks, func(ctx context.Context) error {
			dwdOldSecret := &corev1.Secret{}
			if err := seedClient.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: dwd.InternalProbeSecretName}, dwdOldSecret); err != nil {
				// If ns does not contain old DWD secret, do not procees.
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			// Fetch GRM deployment
			grmDeploy := &appsv1.Deployment{}
			if err := seedClient.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: "gardener-resource-manager"}, grmDeploy); err != nil {
				if apierrors.IsNotFound(err) {
					// Do not proceed if GRM deployment is not present
					return nil
				}
				return err
			}

			// Create a DWDAccess object
			inClusterServerURL := fmt.Sprintf("%s.%s.svc", v1beta1constants.DeploymentNameKubeAPIServer, namespace.Name)
			dwdAccess := dwd.NewAccess(seedClient, namespace.Name, nil, dwd.AccessValues{ServerInCluster: inClusterServerURL})

			if err := dwdAccess.DeployMigrate(ctx); err != nil {
				return err
			}

			// Delete old DWD secrets
			if err := kubernetesutils.DeleteObjects(ctx, seedClient,
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: dwd.InternalProbeSecretName, Namespace: namespace.Name}},
				&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: dwd.ExternalProbeSecretName, Namespace: namespace.Name}},
			); err != nil {
				return err
			}

			// Fetch and update the GRM configmap
			var grmCMName string
			var grmCMVolumeIndex int
			for n, vol := range grmDeploy.Spec.Template.Spec.Volumes {
				if vol.Name == "config" {
					grmCMName = vol.ConfigMap.Name
					grmCMVolumeIndex = n
					break
				}
			}
			if len(grmCMName) == 0 {
				return nil
			}

			grmConfigMap := &corev1.ConfigMap{}
			if err := seedClient.Get(ctx, types.NamespacedName{Namespace: namespace.Name, Name: grmCMName}, grmConfigMap); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return err
			}

			cmData := grmConfigMap.Data["config.yaml"]
			rmConfig := resourcemanagerv1alpha1.ResourceManagerConfiguration{}

			// create codec
			var codec runtime.Codec
			configScheme := runtime.NewScheme()
			utilruntime.Must(resourcemanagerv1alpha1.AddToScheme(configScheme))
			utilruntime.Must(apiextensionsv1.AddToScheme(configScheme))
			ser := json.NewSerializerWithOptions(json.DefaultMetaFactory, configScheme, configScheme, json.SerializerOptions{
				Yaml:   true,
				Pretty: false,
				Strict: false,
			})
			versions := schema.GroupVersions([]schema.GroupVersion{
				resourcemanagerv1alpha1.SchemeGroupVersion,
				apiextensionsv1.SchemeGroupVersion,
			})
			codec = serializer.NewCodecFactory(configScheme).CodecForVersions(ser, ser, versions, versions)

			obj, err := runtime.Decode(codec, []byte(cmData))
			if err != nil {
				return err
			}
			rmConfig = *(obj.(*resourcemanagerv1alpha1.ResourceManagerConfiguration))

			if rmConfig.TargetClientConnection == nil || slices.Contains(rmConfig.TargetClientConnection.Namespaces, corev1.NamespaceNodeLease) {
				return nil
			}

			rmConfig.TargetClientConnection.Namespaces = append(rmConfig.TargetClientConnection.Namespaces, corev1.NamespaceNodeLease)

			data, err := runtime.Encode(codec, &rmConfig)
			if err != nil {
				return err
			}

			newGRMConfigMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "gardener-resource-manager-dwd", Namespace: namespace.Name}}
			newGRMConfigMap.Data = map[string]string{"config.yaml": string(data)}
			utilruntime.Must(kubernetesutils.MakeUnique(newGRMConfigMap))

			if err = seedClient.Create(ctx, newGRMConfigMap); err != nil {
				if !apierrors.IsAlreadyExists(err) {
					return err
				}
			}

			patch := client.MergeFrom(grmDeploy.DeepCopy())
			grmDeploy.Spec.Template.Spec.Volumes[grmCMVolumeIndex].ConfigMap.Name = newGRMConfigMap.Name
			utilruntime.Must(references.InjectAnnotations(grmDeploy))

			return seedClient.Patch(ctx, grmDeploy, patch)
		})
	}
	return flow.Parallel(tasks...)(ctx)
}

// TODO(Kostov6): Remove this code after v1.91 has been released.
func cleanupGRMSecretFinalizers(ctx context.Context, seedClient client.Client, log logr.Logger) error {
	var (
		mrs      = &resourcesv1alpha1.ManagedResourceList{}
		selector = labels.NewSelector()
	)

	// Exclude seed system components while listing
	requirement, err := labels.NewRequirement(v1beta1constants.GardenRole, selection.NotIn, []string{v1beta1constants.GardenRoleSeedSystemComponent})
	if err != nil {
		return fmt.Errorf("failed to construct the requirement: %w", err)
	}
	labelSelector := selector.Add(*requirement)

	if err := seedClient.List(ctx, mrs, client.MatchingLabelsSelector{Selector: labelSelector}); err != nil {
		if meta.IsNoMatchError(err) {
			log.Info("Received a 'no match error' while trying to list managed resources. Will assume that the managed resources CRD is not yet installed (for example new Seed creation) and will skip cleaning up GRM finalizers")
			return nil
		}
		return fmt.Errorf("failed to list managed resources: %w", err)
	}

	return utilclient.ApplyToObjects(ctx, mrs, func(ctx context.Context, obj client.Object) error {
		mr, ok := obj.(*resourcesv1alpha1.ManagedResource)
		if !ok {
			return fmt.Errorf("expected *resourcesv1alpha1.ManagedResource but got %T", obj)
		}

		// only patch MR secrets in shoot namespaces
		if mr.Namespace == v1beta1constants.GardenNamespace {
			return nil
		}

		for _, ref := range mr.Spec.SecretRefs {
			secret := &corev1.Secret{}
			if err := seedClient.Get(ctx, client.ObjectKey{Namespace: mr.Namespace, Name: ref.Name}, secret); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return fmt.Errorf("failed to get secret '%s': %w", kubernetesutils.Key(mr.Namespace, ref.Name), err)
			}

			for _, finalizer := range secret.Finalizers {
				if strings.HasPrefix(finalizer, "resources.gardener.cloud/gardener-resource-manager") {
					if err := controllerutils.RemoveFinalizers(ctx, seedClient, secret, finalizer); err != nil {
						return fmt.Errorf("failed to remove finalizer from secret '%s': %w", client.ObjectKeyFromObject(secret), err)
					}
				}
			}
		}
		return nil
	})
}

// TODO(rfranzke): Remove this code after v1.92 has been released.
func updateShootPrometheusConfigForConnectionToCachePrometheusAndSeedAlertManager(ctx context.Context, seedClient client.Client) error {
	statefulSetList := &appsv1.StatefulSetList{}
	if err := seedClient.List(ctx, statefulSetList, client.MatchingLabels{"app": "prometheus", "role": "monitoring", "gardener.cloud/role": "monitoring"}); err != nil {
		return err
	}

	var taskFns []flow.TaskFn
	for _, obj := range statefulSetList.Items {
		if !strings.HasPrefix(obj.Namespace, v1beta1constants.TechnicalIDPrefix) {
			continue
		}

		statefulSet := obj.DeepCopy()

		taskFns = append(taskFns,
			func(ctx context.Context) error {
				patch := client.MergeFrom(statefulSet.DeepCopy())
				metav1.SetMetaDataLabel(&statefulSet.Spec.Template.ObjectMeta, "networking.resources.gardener.cloud/to-garden-prometheus-cache-tcp-9090", "allowed")
				metav1.SetMetaDataLabel(&statefulSet.Spec.Template.ObjectMeta, "networking.resources.gardener.cloud/to-garden-alertmanager-seed-tcp-9093", "allowed")
				return seedClient.Patch(ctx, statefulSet, patch)
			},
			func(ctx context.Context) error {
				configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-config", Namespace: statefulSet.Namespace}}
				if err := seedClient.Get(ctx, client.ObjectKeyFromObject(configMap), configMap); err != nil {
					if apierrors.IsNotFound(err) {
						return nil
					}
					return err
				}

				if configMap.Data == nil || configMap.Data["prometheus.yaml"] == "" {
					return nil
				}

				patch := client.MergeFrom(configMap.DeepCopy())
				configMap.Data["prometheus.yaml"] = strings.ReplaceAll(configMap.Data["prometheus.yaml"], "prometheus-web.garden.svc", "prometheus-cache.garden.svc")
				return seedClient.Patch(ctx, configMap, patch)
			},
		)
	}

	return flow.Parallel(taskFns...)(ctx)
}

// TODO(shafeeqes): Remove this code after gardener v1.92 has been released.
func cleanupShootCoreManagedResource(ctx context.Context, seedClient client.Client) error {
	shootNamespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, shootNamespaceList, client.MatchingLabels{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot}); err != nil {
		return err
	}

	var taskFns []flow.TaskFn

	for _, ns := range shootNamespaceList.Items {
		namespace := ns

		taskFns = append(taskFns, func(ctx context.Context) error {
			return managedresources.DeleteForShoot(ctx, seedClient, namespace.Name, "shoot-core")
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

// TODO(rfranzke): Remove this code after gardener v1.92 has been released.
func reconcileLabelsForPVCMigrations(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	var (
		labelMigrationNamespace = "disk-migration.monitoring.gardener.cloud/namespace"
		labelMigrationPVCName   = "disk-migration.monitoring.gardener.cloud/pvc-name"
	)

	persistentVolumeList := &corev1.PersistentVolumeList{}
	if err := seedClient.List(ctx, persistentVolumeList, client.HasLabels{labelMigrationPVCName}); err != nil {
		return fmt.Errorf("failed listing persistent volumes with label %s: %w", labelMigrationPVCName, err)
	}

	var (
		persistentVolumeNamesWithoutClaimRef []string
		taskFns                              []flow.TaskFn
	)

	for _, pv := range persistentVolumeList.Items {
		persistentVolume := pv

		if persistentVolume.Labels[labelMigrationNamespace] != "" {
			continue
		}

		if persistentVolume.Status.Phase == corev1.VolumeReleased && persistentVolume.Spec.ClaimRef != nil {
			// check if namespace is already gone - if yes, just clean them up
			if err := seedClient.Get(ctx, client.ObjectKey{Name: persistentVolume.Spec.ClaimRef.Namespace}, &corev1.Namespace{}); err != nil {
				if !apierrors.IsNotFound(err) {
					return fmt.Errorf("failed checking if namespace %s still exists (due to PV %s): %w", persistentVolume.Spec.ClaimRef.Namespace, client.ObjectKeyFromObject(&persistentVolume), err)
				}

				taskFns = append(taskFns, func(ctx context.Context) error {
					log.Info("Deleting orphaned persistent volume in migration", "persistentVolume", client.ObjectKeyFromObject(&persistentVolume))
					return client.IgnoreNotFound(seedClient.Delete(ctx, &persistentVolume))
				})
				continue
			}
		} else if persistentVolume.Spec.ClaimRef == nil {
			persistentVolumeNamesWithoutClaimRef = append(persistentVolumeNamesWithoutClaimRef, persistentVolume.Name)
			continue
		}

		taskFns = append(taskFns, func(ctx context.Context) error {
			log.Info("Adding missing namespace label to persistent volume in migration", "persistentVolume", client.ObjectKeyFromObject(&persistentVolume), "namespace", persistentVolume.Spec.ClaimRef.Namespace)
			patch := client.MergeFrom(persistentVolume.DeepCopy())
			metav1.SetMetaDataLabel(&persistentVolume.ObjectMeta, labelMigrationNamespace, persistentVolume.Spec.ClaimRef.Namespace)
			return seedClient.Patch(ctx, &persistentVolume, patch)
		})
	}

	if err := flow.Parallel(taskFns...)(ctx); err != nil {
		return err
	}

	if len(persistentVolumeNamesWithoutClaimRef) > 0 {
		return fmt.Errorf("found persistent volumes with missing namespace in migration label and `.spec.claimRef=nil` - "+
			"cannot automatically determine the namespace this PV originated from. "+
			"A human operator needs to manually add the namespace and update the label to %s=<namespace> - "+
			"The names of such PVs are: %+v", labelMigrationNamespace, persistentVolumeNamesWithoutClaimRef)
	}

	return nil
}

// TODO: Remove this function when Kubernetes 1.27 support gets dropped.
func migrateDeprecatedTopologyLabels(ctx context.Context, log logr.Logger, seedClient client.Client, restConfig *rest.Config) error {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed creating discovery client: %w", err)
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return fmt.Errorf("failed reading the server version of seed cluster: %w", err)
	}

	seedVersion, err := semver.NewVersion(version.GitVersion)
	if err != nil {
		return fmt.Errorf("failed parsing server version to semver: %w", err)
	}

	//  PV node affinities were immutable until Kubernetes 1.27, see https://github.com/kubernetes/kubernetes/pull/115391
	if !versionutils.ConstraintK8sGreaterEqual127.Check(seedVersion) {
		return nil
	}

	persistentVolumeList := &corev1.PersistentVolumeList{}
	if err := seedClient.List(ctx, persistentVolumeList); err != nil {
		return fmt.Errorf("failed listing persistent volumes for migrating deprecated topology labels: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, pv := range persistentVolumeList.Items {
		persistentVolume := pv

		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(persistentVolume.DeepCopy())

			if persistentVolume.Spec.NodeAffinity == nil {
				// when PV is very old and has no node affinity, we just replace the topology labels
				if v, ok := persistentVolume.Labels[corev1.LabelFailureDomainBetaRegion]; ok {
					persistentVolume.Labels[corev1.LabelTopologyRegion] = v
				}
				if v, ok := persistentVolume.Labels[corev1.LabelFailureDomainBetaZone]; ok {
					persistentVolume.Labels[corev1.LabelTopologyZone] = v
				}
			} else if persistentVolume.Spec.NodeAffinity.Required != nil {
				// when PV has node affinity then we do not need the labels but just need to replace the topology keys
				// in the node selector term match expressions
				for i, term := range persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms {
					for j, expression := range term.MatchExpressions {
						if expression.Key == corev1.LabelFailureDomainBetaRegion {
							persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[j].Key = corev1.LabelTopologyRegion
						}

						if expression.Key == corev1.LabelFailureDomainBetaZone {
							persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[j].Key = corev1.LabelTopologyZone
						}
					}
				}
			}

			// either new topology labels were added above, or node affinity keys were adjusted
			// in both cases, the old, deprecated topology labels are no longer needed and can be removed
			delete(persistentVolume.Labels, corev1.LabelFailureDomainBetaRegion)
			delete(persistentVolume.Labels, corev1.LabelFailureDomainBetaZone)

			// prevent sending empty patches
			if data, err := patch.Data(&persistentVolume); err != nil {
				return fmt.Errorf("failed getting patch data for PV %s: %w", persistentVolume.Name, err)
			} else if string(data) == `{}` {
				return nil
			}

			log.Info("Migrating deprecated topology labels", "persistentVolumeName", persistentVolume.Name)
			return seedClient.Patch(ctx, &persistentVolume, patch)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}
