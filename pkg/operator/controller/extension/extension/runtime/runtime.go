// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/events"
	podsecurityadmissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	chartutils "github.com/gardener/gardener/pkg/utils/chart"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/oci"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

const priorityClassName = v1beta1constants.PriorityClassNameGardenSystem200

// Interface contains functions for the extension deployer in the garden runtime cluster.
type Interface interface {
	// Reconcile creates or updates the extension deployment in the garden runtime cluster.
	Reconcile(context.Context, logr.Logger, kubernetes.Interface, *operatorv1alpha1.Extension) (reconcile.Result, error)
	// Delete deletes the extension deployment in the garden runtime cluster.
	Delete(context.Context, logr.Logger, *operatorv1alpha1.Extension) (reconcile.Result, error)
}

type deployer struct {
	runtimeClientSet kubernetes.Interface
	recorder         events.EventRecorder

	gardenNamespace string
	helmRegistry    oci.Interface
}

// Reconcile creates or updates the extension deployment in the garden runtime cluster.
func (d *deployer) Reconcile(ctx context.Context, log logr.Logger, virtualClusterClientSet kubernetes.Interface, extension *operatorv1alpha1.Extension) (reconcile.Result, error) {
	if !extensionDeploymentSpecified(extension) {
		return d.Delete(ctx, log, extension)
	}

	if err := d.runtimeClientSet.Client().Get(ctx, client.ObjectKey{Name: priorityClassName}, &schedulingv1.PriorityClass{}); err != nil {
		// Prevent exponential backoff of the controller in case the Garden controller is slow in creating the needed
		// PriorityClass when creating a fresh Garden.
		if apierrors.IsNotFound(err) {
			log.Info("PriorityClass not found yet, requeuing", "priorityClassName", priorityClassName)
			return reconcile.Result{RequeueAfter: 2 * time.Second}, fmt.Errorf("priority class %q not found yet", priorityClassName)
		}
		return reconcile.Result{}, fmt.Errorf("failed checking if priority class %q exists: %w", priorityClassName, err)
	}

	requeue, err := d.createOrUpdateResources(ctx, log, virtualClusterClientSet, extension)
	if err != nil {
		return reconcile.Result{}, err
	}
	if requeue {
		return reconcile.Result{RequeueAfter: 2 * time.Second}, nil
	}

	d.recorder.Eventf(extension, nil, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, gardencorev1beta1.EventActionReconcile, "Extension applied successfully in runtime cluster")
	return reconcile.Result{}, nil
}

// Delete deletes the extension deployment in the garden runtime cluster.
func (d *deployer) Delete(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) (reconcile.Result, error) {
	log.Info("Deleting extension resources in garden runtime cluster")
	if err := d.deleteResources(ctx, log, extension); err != nil {
		return reconcile.Result{}, err
	}

	d.recorder.Eventf(extension, nil, corev1.EventTypeNormal, gardencorev1beta1.EventDeleted, gardencorev1beta1.EventActionDelete, "Extension deployment deleted successfully in runtime cluster")
	return reconcile.Result{}, nil
}

func (d *deployer) createOrUpdateResources(ctx context.Context, log logr.Logger, virtualClusterClientSet kubernetes.Interface, extension *operatorv1alpha1.Extension) (bool, error) {
	archive, err := d.helmRegistry.Pull(ctx, extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository)
	if err != nil {
		return false, fmt.Errorf("failed pulling Helm chart from OCI repository %q: %w", extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository.GetURL(), err)
	}

	isSelfHostedShoot, err := gardenerutils.ClusterIsSelfHostedShoot(ctx, d.runtimeClientSet.Client())
	if err != nil {
		return false, fmt.Errorf("failed checking whether the runtime cluster is a self-hosted shoot: %w", err)
	}

	gardenerValues := map[string]any{
		"gardener": map[string]any{
			"clusterTypes": map[string]any{
				v1beta1constants.ClusterTypeGardenRuntimeCluster:   true,
				v1beta1constants.ClusterTypeSelfHostedShootCluster: isSelfHostedShoot,
			},
			"runtimeCluster": map[string]any{
				// TODO(timuthy): Remove this field after Gardener v1.159.0 has been released.
				"enabled":           "true",
				"priorityClassName": priorityClassName,
			},
		},
	}

	var helmValues map[string]any
	if extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues != nil {
		if err := json.Unmarshal(extension.Spec.Deployment.ExtensionDeployment.RuntimeClusterValues.Raw, &helmValues); err != nil {
			return false, err
		}
	}

	helmValues, err = chartutils.RenderHelmValues(ctx, d.runtimeClientSet.Client(), helmValues, d.gardenNamespace, extension.Spec.Deployment.Resources)
	if err != nil {
		return false, fmt.Errorf("failed rendering Helm values for extension deployment: %w", err)
	}

	namespace := operator.ExtensionRuntimeNamespaceName(extension.Name)
	if err := d.ensureNamespace(ctx, namespace, extension); err != nil {
		return false, fmt.Errorf("failed ensuring namespace %q: %w", namespace, err)
	}

	gardenKubeconfigRequired := ptr.Deref(extension.Spec.Deployment.ExtensionDeployment.InjectGardenKubeconfig, false)
	injectGardenKubeconfig := true
	// The virtual garden client is only available during reconciliation (e.g. not while the garden is being deleted).
	// Without it, the generic garden kubeconfig cannot be built, so injection is skipped.
	if !gardenKubeconfigRequired || virtualClusterClientSet == nil {
		log.Info("Skipping garden kubeconfig injection because the virtual garden client is not available yet")
		injectGardenKubeconfig = false
	}

	var genericGardenKubeconfigSecretName string
	if injectGardenKubeconfig {
		genericGardenKubeconfigSecretName, err = d.reconcileGenericGardenKubeconfig(ctx, virtualClusterClientSet, namespace)
		if err != nil {
			return false, fmt.Errorf("failed reconciling generic garden kubeconfig: %w", err)
		}
	}

	renderedChart, err := d.runtimeClientSet.ChartRenderer().RenderArchive(archive, extension.Name, namespace, utils.MergeMaps(helmValues, gardenerValues))
	if err != nil {
		return false, fmt.Errorf("failed rendering Helm chart %q: %w", extension.Spec.Deployment.ExtensionDeployment.Helm.OCIRepository.GetURL(), err)
	}

	secretData := renderedChart.AsSecretData()

	mustInjectGardenKubeconfig, err := gardenerutils.NeedsGardenKubeconfig(injectGardenKubeconfig, secretData)
	if err != nil {
		return false, fmt.Errorf("failed checking whether garden kubeconfig injection is needed: %w", err)
	}

	gardenAccessSecret := gardenerutils.NewGardenAccessSecret("extension", namespace).
		WithServiceAccountLabels(map[string]string{v1beta1constants.LabelKeyExtensionName: extension.Name})

	if mustInjectGardenKubeconfig {
		log.Info("Reconciling garden access secret")
		if err := gardenAccessSecret.Reconcile(ctx, d.runtimeClientSet.Client()); err != nil {
			return false, fmt.Errorf("failed reconciling garden access secret: %w", err)
		}

		if err := gardenerutils.MutateObjectsInSecretData(
			secretData,
			namespace,
			[]string{appsv1.GroupName, batchv1.GroupName},
			// Inject generic garden kubeconfig for virtual garden cluster access.
			func(obj runtime.Object) error {
				if !gardenerutils.ObjectEnablesGardenKubeconfig(obj) {
					return nil
				}
				return gardenerutils.InjectGenericGardenKubeconfig(obj, genericGardenKubeconfigSecretName, gardenAccessSecret.Secret.Name, gardenerutils.VolumeMountPathGenericGardenKubeconfig)
			},
		); err != nil {
			return false, fmt.Errorf("failed injecting garden access secrets: %w", err)
		}
	} else {
		log.Info("Deleting garden access secret")
		if err := client.IgnoreNotFound(d.runtimeClientSet.Client().Delete(ctx, gardenAccessSecret.Secret)); err != nil {
			return false, fmt.Errorf("failed deleting garden access secret: %w", err)
		}
	}

	mrName := operator.ExtensionRuntimeManagedResourceName(extension.Name)
	if err := managedresources.CreateForSeedWithLabels(
		ctx,
		d.runtimeClientSet.Client(),
		d.gardenNamespace,
		mrName,
		false,
		map[string]string{
			managedresources.LabelKeyOrigin:        managedresources.LabelValueOperator,
			v1beta1constants.LabelKeyExtensionName: extension.Name,
		},
		secretData,
	); err != nil {
		return false, fmt.Errorf("failed creating ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilHealthyAndNotProgressing(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName); err != nil {
		return false, fmt.Errorf("failed waiting for ManagedResource to be healthy: %w", err)
	}

	// Request a requeue if the garden kubeconfig is required but not injected yet, so that the next reconciliation can inject it.
	return gardenKubeconfigRequired && !injectGardenKubeconfig, nil
}

// reconcileGenericGardenKubeconfig creates a generic kubeconfig secret in the given namespace that points at the
// in-cluster virtual garden kube-apiserver. The kubeconfig references the garden token file so that it is paired with
// the garden access secret whose token is populated by the operator's token-requestor controller.
func (d *deployer) reconcileGenericGardenKubeconfig(ctx context.Context, virtualClusterClientSet kubernetes.Interface, namespace string) (string, error) {
	restConfig := virtualClusterClientSet.RESTConfig()

	kubeconfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"garden": {
			Server:                   restConfig.Host,
			InsecureSkipTLSVerify:    restConfig.Insecure,
			CertificateAuthorityData: restConfig.CAData,
		}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"extension": {
			TokenFile: gardenerutils.PathGardenToken,
		}},
		Contexts: map[string]*clientcmdapi.Context{"garden": {
			Cluster:  "garden",
			AuthInfo: "extension",
		}},
		CurrentContext: "garden",
	})
	if err != nil {
		return "", fmt.Errorf("failed to serialize generic garden kubeconfig: %w", err)
	}

	kubeconfigSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1beta1constants.SecretNameGenericGardenKubeconfig,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			secretsutils.DataKeyKubeconfig: kubeconfig,
		},
	}

	utilruntime.Must(kubernetesutils.MakeUnique(kubeconfigSecret))

	return kubeconfigSecret.Name, client.IgnoreAlreadyExists(d.runtimeClientSet.Client().Create(ctx, kubeconfigSecret))
}

func (d *deployer) deleteResources(ctx context.Context, log logr.Logger, extension *operatorv1alpha1.Extension) error {
	mrName := operator.ExtensionRuntimeManagedResourceName(extension.Name)
	namespace := operator.ExtensionRuntimeNamespaceName(extension.Name)

	log.Info("Deleting extension ManagedResource for runtime cluster if present", "managedResource", client.ObjectKey{Name: mrName, Namespace: d.gardenNamespace})
	if err := client.IgnoreNotFound(managedresources.DeleteForSeed(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName)); err != nil {
		return fmt.Errorf("failed deleting ManagedResource: %w", err)
	}

	if err := managedresources.WaitUntilDeleted(ctx, d.runtimeClientSet.Client(), d.gardenNamespace, mrName); err != nil {
		return fmt.Errorf("failed waiting for ManagedResource to be deleted: %w", err)
	}

	if err := d.deleteNamespace(ctx, namespace); err != nil {
		return fmt.Errorf("failed deleting namespace %q: %w", namespace, err)
	}

	return nil
}

func (d *deployer) ensureNamespace(ctx context.Context, name string, extension *operatorv1alpha1.Extension) error {
	gardenNamespace := &corev1.Namespace{}
	if err := d.runtimeClientSet.Client().Get(ctx, client.ObjectKey{Name: d.gardenNamespace}, gardenNamespace); err != nil {
		return err
	}
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, d.runtimeClientSet.Client(), namespace, func() error {
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.GardenRole, v1beta1constants.GardenRoleExtension)
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigConsider, "true")
		metav1.SetMetaDataLabel(&namespace.ObjectMeta, v1beta1constants.LabelNetworkPolicyAccessTargetAPIServer, "allowed")

		if zones := gardenNamespace.Annotations[resourcesv1alpha1.HighAvailabilityConfigZones]; zones != "" {
			metav1.SetMetaDataAnnotation(&namespace.ObjectMeta, resourcesv1alpha1.HighAvailabilityConfigZones, zones)
		} else {
			delete(namespace.Annotations, resourcesv1alpha1.HighAvailabilityConfigZones)
		}

		if podSecurityEnforce, ok := extension.Annotations[v1beta1constants.AnnotationPodSecurityEnforce]; ok {
			metav1.SetMetaDataLabel(&namespace.ObjectMeta, podsecurityadmissionapi.EnforceLevelLabel, podSecurityEnforce)
		} else {
			delete(namespace.Labels, podsecurityadmissionapi.EnforceLevelLabel)
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (d *deployer) deleteNamespace(ctx context.Context, name string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := client.IgnoreNotFound(d.runtimeClientSet.Client().Delete(ctx, namespace)); err != nil {
		return err
	}
	return kubernetesutils.WaitUntilResourceDeleted(ctx, d.runtimeClientSet.Client(), namespace, time.Second)
}

func extensionDeploymentSpecified(extension *operatorv1alpha1.Extension) bool {
	return extension.Spec.Deployment != nil &&
		extension.Spec.Deployment.ExtensionDeployment != nil &&
		extension.Spec.Deployment.ExtensionDeployment.Helm != nil
}

// New creates a new runtime deployer.
func New(runtimeClientSet kubernetes.Interface, recorder events.EventRecorder, gardenNamespace string, registry oci.Interface) Interface {
	return &deployer{
		runtimeClientSet: runtimeClientSet,
		recorder:         recorder,
		gardenNamespace:  gardenNamespace,
		helmRegistry:     registry,
	}
}
