// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/nodemanagement/machinecontrollermanager"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, gardenClient client.Client) error {
	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient()); err != nil {
		return err
	}

	if features.DefaultFeatureGate.Enabled(features.RemoveAPIServerProxyLegacyPort) {
		if err := verifyRemoveAPIServerProxyLegacyPortFeatureGate(ctx, gardenClient, g.config.SeedConfig.Name); err != nil {
			return err
		}
	}

	log.Info("Migrating RBAC resources for machine-controller-manager")
	if err := migrateMCMRBAC(ctx, g.mgr.GetClient()); err != nil {
		return err
	}

	// TODO(shafeeqes): Remove this in gardener v1.125
	{
		log.Info("Deleting ingress controller resource lock configmaps")
		if err := kubernetesutils.DeleteObjects(ctx, g.mgr.GetClient(),
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-controller-seed-leader",
					Namespace: v1beta1constants.GardenNamespace,
				},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-controller-seed-leader-nginx-gardener",
					Namespace: v1beta1constants.GardenNamespace,
				},
			},
		); err != nil {
			return fmt.Errorf("failed deleting ingress controller resource lock configmaps: %w", err)
		}
	}

	return nil
}

// TODO: Remove this function when Kubernetes 1.27 support gets dropped.
func migrateDeprecatedTopologyLabels(ctx context.Context, log logr.Logger, seedClient client.Client) error {
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

// TODO(Wieneo): Remove this function when feature gate RemoveAPIServerProxyLegacyPort is removed
func verifyRemoveAPIServerProxyLegacyPortFeatureGate(ctx context.Context, gardenClient client.Client, seedName string) error {
	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList); err != nil {
		return err
	}

	for _, k := range shootList.Items {
		if specSeedName, statusSeedName := gardenerutils.GetShootSeedNames(&k); gardenerutils.GetResponsibleSeedName(specSeedName, statusSeedName) != seedName {
			continue
		}

		// we need to ignore shoots under the following conditions:
		// - it is workerless
		// - it is not yet picked up by gardenlet or still in phase "Creating"
		//
		// this is needed bcs. the constraint "ShootAPIServerProxyUsesHTTPProxy" is only set once the apiserver-proxy component is deployed to the shoot
		// this will never happen if the shoot is workerless or the component could still be missing, if the gardenlet is restarted during the creation of a shoot
		if v1beta1helper.IsWorkerless(&k) {
			continue
		}

		if k.Status.LastOperation == nil || ((k.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate || k.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeDelete) && k.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded) {
			continue
		}

		if cond := v1beta1helper.GetCondition(k.Status.Constraints, gardencorev1beta1.ShootAPIServerProxyUsesHTTPProxy); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
			return errors.New("the `proxy` port on the istio ingress gateway cannot be removed until all api server proxies in all shoots on this seed have been reconfigured to use the `tls-tunnel` port instead, i.e., the `RemoveAPIServerProxyLegacyPort` feature gate can only be enabled once all shoots have the `APIServerProxyUsesHTTPProxy` constraint with status `true`")
		}
	}

	return nil
}

// syncBackupSecretRefAndCredentialsRef syncs the seed backup credentials when possible.
// TODO(vpnachev): Remove this function after v1.121.0 has been released.
func syncBackupSecretRefAndCredentialsRef(backup *gardencorev1beta1.Backup) {
	if backup == nil {
		return
	}

	emptySecretRef := corev1.SecretReference{}

	// secretRef is set and credentialsRef is not, sync both fields.
	if backup.SecretRef != emptySecretRef && backup.CredentialsRef == nil {
		backup.CredentialsRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  backup.SecretRef.Namespace,
			Name:       backup.SecretRef.Name,
		}

		return
	}

	// secretRef is unset and credentialsRef refer a secret, sync both fields.
	if backup.SecretRef == emptySecretRef && backup.CredentialsRef != nil &&
		backup.CredentialsRef.APIVersion == "v1" && backup.CredentialsRef.Kind == "Secret" {
		backup.SecretRef = corev1.SecretReference{
			Namespace: backup.CredentialsRef.Namespace,
			Name:      backup.CredentialsRef.Name,
		}

		return
	}

	// in all other cases we can do nothing:
	// - both fields are unset -> we have nothing to sync
	// - both fields are set -> let the validation check if they are correct
	// - credentialsRef refer to WorkloadIdentity -> secretRef should stay unset
}

// TODO(@aaronfern): Remove this after v1.123 is released
func migrateMCMRBAC(ctx context.Context, seedClient client.Client) error {
	namespaceList := &corev1.NamespaceList{}
	if err := seedClient.List(ctx, namespaceList, client.MatchingLabels(map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot})); err != nil {
		return fmt.Errorf("failed listing namespaces with '%s: %s' label: %w", v1beta1constants.GardenRole, v1beta1constants.GardenRoleShoot, err)
	}

	var tasks []flow.TaskFn

	for _, namespace := range namespaceList.Items {
		if namespace.DeletionTimestamp != nil || namespace.Status.Phase == corev1.NamespaceTerminating {
			continue
		}
		tasks = append(tasks, func(ctx context.Context) error {
			clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
			if err := seedClient.Get(ctx, client.ObjectKey{Name: "machine-controller-manager-" + namespace.Name}, clusterRoleBinding); err != nil {
				//If MCM clusterRoleBinding does not exist, nothing to do
				return client.IgnoreNotFound(err)
			}

			return machinecontrollermanager.New(seedClient, namespace.Name, nil, machinecontrollermanager.Values{}).MigrateRBAC(ctx)
		})
	}

	if err := flow.Parallel(tasks...)(ctx); err != nil {
		return err
	}
	if err := managedresources.DeleteForSeed(ctx, seedClient, "garden", "machine-controller-manager"); err != nil {
		if !meta.IsNoMatchError(err) {
			return err
		}
	}
	return nil
}
