// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/controller/extension"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubernetesclient "github.com/gardener/gardener/pkg/client/kubernetes"
	kubeproxy "github.com/gardener/gardener/pkg/component/kubernetes/proxy"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

const (
	// ApplicationName is the name of the application.
	ApplicationName string = "local-ext-shoot"
	// ManagedResourceNamesShoot is the name used to describe the managed shoot resources.
	ManagedResourceNamesShoot string = ApplicationName
	finalizer                 string = "extensions.gardener.cloud/local-ext-shoot"
	// AnnotationTestForceDeleteShoot is an annotation used in the force-deletion e2e test which makes this actuator
	// deploy two empty NetworkPolicies with a finalizer.
	AnnotationTestForceDeleteShoot string = "test-force-delete"
)

type actuator struct {
	client client.Client
}

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator(mgr manager.Manager) extension.Actuator {
	return &actuator{
		client: mgr.GetClient(),
	}
}

// Reconcile the extension resource.
func (a *actuator) Reconcile(ctx context.Context, _ logr.Logger, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.Namespace

	resources, err := getResources()
	if err != nil {
		return err
	}

	var (
		injectedLabels       = map[string]string{v1beta1constants.ShootNoCleanup: "true"}
		secretNameWithPrefix = true
		keepObjects          = false
	)

	if err := managedresources.Create(
		ctx,
		a.client,
		namespace,
		ManagedResourceNamesShoot,
		map[string]string{},
		secretNameWithPrefix,
		"",
		resources,
		&keepObjects,
		injectedLabels,
		nil,
	); err != nil {
		return err
	}

	if isShootNamespace, err := gardenerutils.IsShootNamespace(ctx, a.client, ex.Namespace); err != nil {
		return fmt.Errorf("error checking if Extension is in a shoot namespace: %w", err)
	} else if isShootNamespace {
		cluster, err := extensionscontroller.GetCluster(ctx, a.client, ex.Namespace)
		if err != nil {
			return err
		}

		// Create the resources only for force-delete e2e test
		if kubernetesutils.HasMetaDataAnnotation(cluster.Shoot, AnnotationTestForceDeleteShoot, "true") {
			for i := 1; i <= 2; i++ {
				networkPolicy := &networkingv1.NetworkPolicy{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "test-netpol-",
						Namespace:    ex.Namespace,
						Finalizers:   []string{finalizer},
					},
				}

				if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, a.client, networkPolicy, func() error {
					networkPolicy.Labels = getLabels()
					return nil
				}); err != nil {
					return err
				}
			}
		}

		// Trigger immediate reconciliation of kube-proxy ManagedResources so that the shoot webhook
		// (which sets conntrack MaxPerCore=0) fires for existing kube-proxy ConfigMaps that were
		// created before the webhook was registered in the shoot cluster.
		if err := a.triggerKubeProxyManagedResourceReconciliation(ctx, namespace); err != nil {
			return err
		}
	}

	return nil
}

// Delete the extension resource.
func (a *actuator) Delete(ctx context.Context, _ logr.Logger, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.GetNamespace()
	twoMinutes := 2 * time.Minute

	timeoutShootCtx, cancelShootCtx := context.WithTimeout(ctx, twoMinutes)
	defer cancelShootCtx()

	if err := managedresources.DeleteForShoot(ctx, a.client, namespace, ManagedResourceNamesShoot); err != nil {
		return err
	}

	return managedresources.WaitUntilDeleted(timeoutShootCtx, a.client, namespace, ManagedResourceNamesShoot)
}

// ForceDelete force deletes the extension resource.
func (a *actuator) ForceDelete(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	log.Info("Deleting all test NetworkPolicies in namespace", "namespace", ex.Namespace)
	return flow.Parallel(
		utilclient.ForceDeleteObjects(a.client, ex.Namespace, &networkingv1.NetworkPolicyList{}, client.MatchingLabels(getLabels())),
	)(ctx)
}

// Migrate the extension resource.
func (a *actuator) Migrate(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	// Keep objects for shoot managed resources so that they are not deleted from the shoot during the migration
	if err := managedresources.SetKeepObjects(ctx, a.client, ex.GetNamespace(), ManagedResourceNamesShoot, true); err != nil {
		return err
	}

	return a.Delete(ctx, log, ex)
}

// Restore the extension resource.
func (a *actuator) Restore(ctx context.Context, log logr.Logger, ex *extensionsv1alpha1.Extension) error {
	return a.Reconcile(ctx, log, ex)
}

func getLabels() map[string]string {
	return map[string]string{"app.kubernetes.io/name": ApplicationName}
}

// triggerKubeProxyManagedResourceReconciliation annotates all kube-proxy ManagedResources with the
// gardener.cloud/operation=reconcile annotation. This causes the resource manager to immediately
// reconcile them, triggering the shoot webhook for kube-proxy ConfigMaps even if they were
// created before the webhook was registered in the shoot cluster.
func (a *actuator) triggerKubeProxyManagedResourceReconciliation(ctx context.Context, namespace string) error {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := a.client.List(ctx, mrList, client.InNamespace(namespace), kubeproxy.ManagedResourceLabelSelector()); err != nil {
		return fmt.Errorf("failed to list kube-proxy ManagedResources: %w", err)
	}

	for i := range mrList.Items {
		mr := &mrList.Items[i]
		patch := client.MergeFrom(mr.DeepCopy())
		metav1.SetMetaDataAnnotation(&mr.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		if err := a.client.Patch(ctx, mr, patch); err != nil {
			return fmt.Errorf("failed to annotate kube-proxy ManagedResource %q: %w", mr.Name, err)
		}
	}

	return nil
}

func getResources() (map[string][]byte, error) {
	shootRegistry := managedresources.NewRegistry(kubernetesclient.ShootScheme, kubernetesclient.ShootCodec, kubernetesclient.ShootSerializer)
	return shootRegistry.AddAllAndSerialize(
		&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ApplicationName,
				Namespace: metav1.NamespaceSystem,
				Labels:    getLabels(),
			},
			AutomountServiceAccountToken: new(false),
		},
	)
}
