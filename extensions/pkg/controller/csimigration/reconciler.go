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

package csimigration

import (
	"context"
	"time"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/version"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// RequeueAfter is the duration to requeue a Cluster reconciliation if indicated by the CSI controller.
const RequeueAfter = time.Minute

type reconciler struct {
	logger logr.Logger

	client  client.Client
	decoder runtime.Decoder

	csiMigrationKubernetesVersion       string
	storageClassNameToLegacyProvisioner map[string]string
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Cluster resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(csiMigrationKubernetesVersion string, storageClassNameToLegacyProvisioner map[string]string) reconcile.Reconciler {
	return &reconciler{
		logger:                              log.Log.WithName(ControllerName),
		decoder:                             extensionscontroller.NewGardenDecoder(),
		csiMigrationKubernetesVersion:       csiMigrationKubernetesVersion,
		storageClassNameToLegacyProvisioner: storageClassNameToLegacyProvisioner,
	}
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	cluster := &extensionsv1alpha1.Cluster{}
	if err := r.client.Get(ctx, request.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	shoot, err := extensionscontroller.ShootFromCluster(r.decoder, cluster)
	if err != nil {
		return reconcile.Result{}, err
	}

	if extensionscontroller.IsShootFailed(shoot) {
		r.logger.Info("Skipping the reconciliation of csimigration of failed shoot", "csimigration", kutil.ObjectName(shoot))
		return reconcile.Result{}, nil
	}

	if cluster.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	r.logger.Info("CSI migration controller got called with cluster", "csimigration", kutil.ObjectName(cluster))

	return r.reconcile(ctx, cluster, shoot)
}

// NewClientForShoot is a function to create a new client for shoots.
var NewClientForShoot = util.NewClientForShoot

func (r *reconciler) reconcile(ctx context.Context, cluster *extensionsv1alpha1.Cluster, shoot *gardencorev1beta1.Shoot) (reconcile.Result, error) {
	if !metav1.HasAnnotation(cluster.ObjectMeta, AnnotationKeyNeedsComplete) {
		// Check if a ControlPlane object exists for the cluster. If false then it is a new shoot that was created with
		// at least the minimum Kubernetes version that is used for CSI migration. In this case we can directly set the
		// CSIMigration<Provider>Complete annotations and proceed.
		if err := r.client.Get(ctx, kutil.Key(cluster.Name, shoot.Name), &extensionsv1alpha1.ControlPlane{}); err != nil {
			if !apierrors.IsNotFound(err) {
				return reconcile.Result{}, err
			}

			r.logger.Info("CSI migration controller detected new shoot cluster with minimum CSI migration Kubernetes version - adding both annotations", "cismigration", kutil.ObjectName(cluster))

			metav1.SetMetaDataAnnotation(&cluster.ObjectMeta, AnnotationKeyNeedsComplete, "true")
			metav1.SetMetaDataAnnotation(&cluster.ObjectMeta, AnnotationKeyControllerFinished, "true")
			return reconcile.Result{}, r.client.Update(ctx, cluster)
		}

		kubernetesVersionForCSIMigration := r.csiMigrationKubernetesVersion
		if overwrite, ok := shoot.Annotations[extensionsv1alpha1.ShootAlphaCSIMigrationKubernetesVersion]; ok {
			kubernetesVersionForCSIMigration = overwrite
		}

		k8sVersionIsMinimum, err := version.CompareVersions(shoot.Spec.Kubernetes.Version, "~", kubernetesVersionForCSIMigration)
		if err != nil {
			return reconcile.Result{}, err
		}

		// At this point the version is either lower, equal, or higher than the minimum version that introduces CSI. It
		// cannot be lower as this is prevented via the controller's predicates. It also cannot be higher because then
		// it was either newly created (but then it only has seen above code and would never reach this step), or it was
		// regularly updated from the minimum version, but then it has already seen the migration code below. Hence, if
		// the Kubernetes version does not match the minimum version we have nothing to do anymore. This case should be
		// basically unreachable.
		if !k8sVersionIsMinimum {
			return reconcile.Result{}, nil
		}

		// At this point the version is equal to the minimum Kubernetes version that introduces CSI, hence, let's start
		// our migration flow.
		r.logger.Info("CSI migration controller detected existing shoot cluster with minimum Kubernetes version - starting migration", "csimigrations", kutil.ObjectName(cluster))

		// If the shoot is hibernated then we wait until the cluster gets woken up again so that the kube-controller-manager
		// can perform the CSI migration steps.
		if extensionscontroller.IsHibernated(&extensionscontroller.Cluster{Shoot: shoot}) {
			r.logger.Info("Shoot cluster is hibernated - doing nothing until it gets woken up", "csimigration", kutil.ObjectName(cluster))
			return reconcile.Result{}, nil
		}

		_, shootClient, err := NewClientForShoot(ctx, r.client, cluster.Name, client.Options{})
		if err != nil {
			return reconcile.Result{}, err
		}

		// Checking all the nodes - if only nodes running a kubelet of the minimum Kubernetes version exist in the
		// cluster the CSI migration is considered completed.
		nodeList := &corev1.NodeList{}
		if err := shootClient.List(ctx, nodeList); err != nil {
			return reconcile.Result{}, err
		}

		for _, node := range nodeList.Items {
			kubeletVersionAtLeastMinimum, err := version.CompareVersions(node.Status.NodeInfo.KubeletVersion, ">=", kubernetesVersionForCSIMigration)
			if err != nil {
				return reconcile.Result{}, err
			}

			// At least one kubelet is of a version lower than our minimum version - requeueing and waiting until all
			// kubelets are updated.
			if !kubeletVersionAtLeastMinimum {
				r.logger.Info("At least one kubelet was not yet updated to the minimum Kubernetes version - requeuing", "csimigration", kutil.ObjectName(cluster), "nodeName", node.Name)
				return reconcile.Result{RequeueAfter: RequeueAfter}, nil
			}
		}

		// Delete legacy storage classes created by the extension controller to allow their recreation with the new CSI
		// provisioner names and the same storage class names (the storage classes are immutable, hence, a regular UPDATE
		// does not work).
		storageClassList := &storagev1.StorageClassList{}
		if err := shootClient.List(ctx, storageClassList); err != nil {
			return reconcile.Result{}, err
		}

		for _, storageClass := range storageClassList.Items {
			if legacyProvisioner, ok := r.storageClassNameToLegacyProvisioner[storageClass.Name]; ok && storageClass.Provisioner == legacyProvisioner {
				r.logger.Info("Deleting storage class using legacy provisioner", "csimigration", kutil.ObjectName(cluster), "storageclassname", storageClass.Name)
				if err := shootClient.Delete(ctx, storageClass.DeepCopy()); client.IgnoreNotFound(err) != nil {
					return reconcile.Result{}, err
				}
			}
		}

		// At this point the migration has been finished. We are updating the annotation and then send out empty PATCH
		// requests against the Kubernetes control plane component deployments so that the provider-specific webhooks
		// can adapt the injected feature gates.
		metav1.SetMetaDataAnnotation(&cluster.ObjectMeta, AnnotationKeyNeedsComplete, "true")
		if err := r.client.Update(ctx, cluster); err != nil {
			return reconcile.Result{}, err
		}
	}

	for _, deploymentName := range []string{
		v1beta1constants.DeploymentNameKubeAPIServer,
		v1beta1constants.DeploymentNameKubeControllerManager,
		v1beta1constants.DeploymentNameKubeScheduler,
	} {
		r.logger.Info("Submitting empty PATCH for control plane component deployment", "csimigration", kutil.ObjectName(cluster), "deploymentName", deploymentName)

		obj := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deploymentName,
				Namespace: cluster.Name,
			},
		}

		// submit empty patch
		if err := r.client.Patch(ctx, obj, client.RawPatch(types.StrategicMergePatchType, []byte("{}"))); err != nil {
			return reconcile.Result{}, err
		}
	}

	r.logger.Info("CSI migration completed successfully", "csimigration", kutil.ObjectName(cluster))

	metav1.SetMetaDataAnnotation(&cluster.ObjectMeta, AnnotationKeyControllerFinished, "true")
	return reconcile.Result{}, r.client.Update(ctx, cluster)
}
