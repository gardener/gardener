// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/garden/projectrbac"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Projects.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.ProjectControllerConfiguration
	Recorder record.EventRecorder

	// RateLimiter allows limiting exponential backoff for testing purposes
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// Reconcile reconciles Projects.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	project := &gardencorev1beta1.Project{}
	if err := r.Client.Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if project.DeletionTimestamp != nil {
		log.Info("Deleting project")
		return r.delete(ctx, log, project)
	}

	log.Info("Reconciling project")
	return reconcile.Result{}, r.reconcile(ctx, log, project)
}

func patchProjectPhase(ctx context.Context, c client.Client, project *gardencorev1beta1.Project, phase gardencorev1beta1.ProjectPhase) error {
	patch := client.StrategicMergeFrom(project.DeepCopy())
	project.Status.ObservedGeneration = project.Generation
	project.Status.Phase = phase
	return c.Status().Patch(ctx, project, patch)
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, project *gardencorev1beta1.Project) error {
	if !controllerutil.ContainsFinalizer(project, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, project, gardencorev1beta1.GardenerName); err != nil {
			return fmt.Errorf("could not add finalizer: %w", err)
		}
	}

	// If the project has no phase yet then we update it to be 'pending'.
	if len(project.Status.Phase) == 0 {
		if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectPending); err != nil {
			return err
		}
	}

	ownerReference := metav1.NewControllerRef(project, gardencorev1beta1.SchemeGroupVersion.WithKind("Project"))

	// reconcile the namespace for the project:
	// - if the .spec.namespace is set, try to adopt it
	// - if it is not set, determine the namespace name based on project UID and create it
	namespace, err := r.reconcileNamespaceForProject(ctx, log, project, ownerReference)
	if err != nil {
		r.Recorder.Event(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, err.Error())
		if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectFailed); err != nil {
			log.Error(err, "Failed to update Project status")
		}
		return err
	}
	r.Recorder.Eventf(project, corev1.EventTypeNormal, gardencorev1beta1.ProjectEventNamespaceReconcileSuccessful, "Successfully reconciled namespace %q for project", namespace.Name)

	// set the created namespace in spec.namespace
	if project.Spec.Namespace == nil {
		project.Spec.Namespace = ptr.To(namespace.Name)
		if err := r.Client.Update(ctx, project); err != nil {
			r.Recorder.Event(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, err.Error())
			if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectFailed); err != nil {
				log.Error(err, "Failed to update Project status")
			}
			return err
		}
	}

	// Create ResourceQuota for project if configured.
	quotaConfig, err := quotaConfigurationForProject(r.Config, project)
	if err != nil {
		r.Recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while setting up ResourceQuota: %+v", err)
		if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectFailed); err != nil {
			log.Error(err, "Failed to update Project status")
		}
		return err
	}

	if quotaConfig != nil {
		if err := createOrUpdateResourceQuota(ctx, r.Client, namespace.Name, ownerReference, *quotaConfig); err != nil {
			r.Recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while setting up ResourceQuota: %+v", err)
			if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectFailed); err != nil {
				log.Error(err, "Failed to update Project status")
			}
			return err
		}
	}

	// Create RBAC rules to allow project members to interact with it.
	rbac, err := projectrbac.New(r.Client, project)
	if err != nil {
		r.Recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while preparing for reconciling RBAC resources for namespace %q: %+v", namespace.Name, err)
		if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectFailed); err != nil {
			log.Error(err, "Failed to update Project status")
		}
		return err
	}

	if err := rbac.Deploy(ctx); err != nil {
		r.Recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while reconciling RBAC resources for namespace %q: %+v", namespace.Name, err)
		if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectFailed); err != nil {
			log.Error(err, "Failed to update Project status")
		}
		return err
	}

	if err := rbac.DeleteStaleExtensionRolesResources(ctx); err != nil {
		r.Recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while deleting stale RBAC rules for extension roles: %+v", err)
		if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectFailed); err != nil {
			log.Error(err, "Failed to update Project status")
		}
		return err
	}

	// Update the project status to mark it as 'ready'.
	if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectReady); err != nil {
		r.Recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceReconcileFailed, "Error while trying to mark project as ready: %+v", err)
		return err
	}

	return nil
}

func (r *Reconciler) reconcileNamespaceForProject(ctx context.Context, log logr.Logger, project *gardencorev1beta1.Project, ownerReference *metav1.OwnerReference) (*corev1.Namespace, error) {
	var (
		namespaceName = ptr.Deref(project.Spec.Namespace, "")

		projectLabels      = namespaceLabelsFromProject(project)
		projectAnnotations = namespaceAnnotationsFromProject(project)
	)

	if namespaceName == "" {
		// Use a deterministic namespace name per Project instance if not specified.
		// This is better than GenerateName, because we don't need to worry about saving the generated namespace name in the
		// Project spec to prevent creating orphaned Namespaces. If we fail to update the Project spec with this namespace
		// name though, we will yield the same name on the every reconciliation of the same Project instance.
		// Also, if the Project gets deleted before we could update spec.namespace, the garbage collector will take care of
		// cleaning up the created namespace.
		// In comparison to GenerateName, it might however happen, that the determined project namespace is already used by
		// a different Project. That's why we try to create the namespace first before updating spec.namespace.
		// If the namespace is already taken, the Project will stay in Failed state but the owner can update spec.namespace
		// to an arbitrary namespace to unblock their Project.
		namespaceName = fmt.Sprintf("%s%s-%s", gardenerutils.ProjectNamespacePrefix, project.Name, utils.ComputeSHA256Hex([]byte(project.UID))[:5])
	}

	namespace := &corev1.Namespace{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: namespaceName}, namespace); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}

		obj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:            namespaceName,
				OwnerReferences: []metav1.OwnerReference{*ownerReference},
				Labels:          projectLabels,
				Annotations:     projectAnnotations,
			},
		}
		obj.Annotations[v1beta1constants.NamespaceCreatedByProjectController] = "true"

		log.Info("Creating Namespace for Project", "namespaceName", obj.Name)
		return obj, r.Client.Create(ctx, obj)
	}

	if !apiequality.Semantic.DeepDerivative(projectLabels, namespace.Labels) {
		return nil, fmt.Errorf("namespace cannot be used as it needs the project labels %#v", projectLabels)
	}

	if metav1.HasAnnotation(namespace.ObjectMeta, v1beta1constants.NamespaceProject) && !apiequality.Semantic.DeepDerivative(projectAnnotations, namespace.Annotations) {
		return nil, errors.New("namespace is already in-use by another project")
	}

	before := namespace.DeepCopy()

	namespace.OwnerReferences = kubernetesutils.MergeOwnerReferences(namespace.OwnerReferences, *ownerReference)
	namespace.Labels = utils.MergeStringMaps(namespace.Labels, projectLabels)
	namespace.Annotations = utils.MergeStringMaps(namespace.Annotations, projectAnnotations)

	// Add the "keep-after-project-deletion" annotation to the namespace only when we adopt it
	// (i.e. the namespace was not created by the project controller).
	if !metav1.HasAnnotation(namespace.ObjectMeta, v1beta1constants.NamespaceCreatedByProjectController) {
		namespace.Annotations[v1beta1constants.NamespaceKeepAfterProjectDeletion] = "true"
	}

	if apiequality.Semantic.DeepEqual(before, namespace) {
		return namespace, nil
	}

	log.Info("Adopting Namespace for Project", "namespaceName", namespace.Name)
	return namespace, r.Client.Update(ctx, namespace)
}

// quotaConfigurationForProject returns the first matching quota configuration if one is configured for the given project.
func quotaConfigurationForProject(config controllermanagerconfigv1alpha1.ProjectControllerConfiguration, project *gardencorev1beta1.Project) (*controllermanagerconfigv1alpha1.QuotaConfiguration, error) {
	for _, c := range config.Quotas {
		quotaConfig := c
		selector, err := metav1.LabelSelectorAsSelector(quotaConfig.ProjectSelector)
		if err != nil {
			return nil, err
		}

		if selector.Matches(labels.Set(project.GetLabels())) {
			return &quotaConfig, nil
		}
	}

	return nil, nil
}

// ResourceQuotaName is the name of the default ResourceQuota resource that is created by Gardener in the project namespace.
const ResourceQuotaName = "gardener"

func createOrUpdateResourceQuota(ctx context.Context, c client.Client, projectNamespace string, ownerReference *metav1.OwnerReference, config controllermanagerconfigv1alpha1.QuotaConfiguration) error {
	projectResourceQuota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ResourceQuotaName,
			Namespace: projectNamespace,
		},
	}

	if _, err := controllerutils.GetAndCreateOrStrategicMergePatch(ctx, c, projectResourceQuota, func() error {
		projectResourceQuota.SetOwnerReferences(kubernetesutils.MergeOwnerReferences(projectResourceQuota.GetOwnerReferences(), *ownerReference))
		projectResourceQuota.Labels = utils.MergeStringMaps(projectResourceQuota.Labels, config.Config.Labels)
		projectResourceQuota.Annotations = utils.MergeStringMaps(projectResourceQuota.Annotations, config.Config.Annotations)
		quotas := make(map[corev1.ResourceName]resource.Quantity)
		for resourceName, quantity := range config.Config.Spec.Hard {
			if val, ok := projectResourceQuota.Spec.Hard[resourceName]; ok {
				// Do not overwrite already existing quotas.
				quotas[resourceName] = val
				continue
			}
			quotas[resourceName] = quantity
		}
		projectResourceQuota.Spec.Hard = quotas
		return nil
	}); err != nil {
		return err
	}

	return nil
}

func namespaceLabelsFromProject(project *gardencorev1beta1.Project) map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole:  v1beta1constants.GardenRoleProject,
		v1beta1constants.ProjectName: project.Name,
	}
}

func namespaceAnnotationsFromProject(project *gardencorev1beta1.Project) map[string]string {
	return map[string]string{
		v1beta1constants.NamespaceProject: string(project.UID),
	}
}

func (r *Reconciler) delete(ctx context.Context, log logr.Logger, project *gardencorev1beta1.Project) (reconcile.Result, error) {
	if !controllerutil.ContainsFinalizer(project, gardencorev1beta1.GardenerName) {
		return reconcile.Result{}, nil
	}

	if namespace := project.Spec.Namespace; namespace != nil {
		log = log.WithValues("namespaceName", *namespace)

		inUse, err := kubernetesutils.ResourcesExist(ctx, r.Client, &gardencorev1beta1.ShootList{}, r.Client.Scheme(), client.InNamespace(*namespace))
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to check if namespace is empty: %w", err)
		}

		if inUse {
			r.Recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceNotEmpty, "Cannot release namespace %q because it still contains Shoots", *namespace)
			log.Info("Cannot release Project Namespace because it still contains Shoots")
			return reconcile.Result{Requeue: true}, patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectTerminating)
		}

		released, err := r.releaseNamespace(ctx, log, project, *namespace)
		if err != nil {
			r.Recorder.Eventf(project, corev1.EventTypeWarning, gardencorev1beta1.ProjectEventNamespaceDeletionFailed, "Failed to release project namespace %q: %v", *namespace, err)
			if err := patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectFailed); err != nil {
				log.Error(err, "Failed to update Project status")
			}
			return reconcile.Result{}, fmt.Errorf("failed to release project namespace: %w", err)
		}

		if !released {
			r.Recorder.Eventf(project, corev1.EventTypeNormal, gardencorev1beta1.ProjectEventNamespaceMarkedForDeletion, "Successfully marked project namespace %q for deletion", *namespace)
			// Project will be enqueued again once project namespace is gone, but recheck every minute to be sure
			return reconcile.Result{RequeueAfter: time.Minute}, patchProjectPhase(ctx, r.Client, project, gardencorev1beta1.ProjectTerminating)
		}
	}

	log.Info("Removing finalizer")
	if err := controllerutils.RemoveFinalizers(ctx, r.Client, project, gardencorev1beta1.GardenerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}

	return reconcile.Result{}, nil
}

func (r *Reconciler) releaseNamespace(ctx context.Context, log logr.Logger, project *gardencorev1beta1.Project, namespaceName string) (bool, error) {
	namespace := &corev1.Namespace{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: namespaceName}, namespace); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}

	// If the namespace has been already marked for deletion we do not need to do it again.
	if namespace.DeletionTimestamp != nil {
		log.Info("Project Namespace is already marked for deletion, nothing to do for releasing it")
		return false, nil
	}

	// To prevent "stealing" namespaces by other projects we only delete the namespace if its labels match
	// the project labels.
	if !apiequality.Semantic.DeepDerivative(namespaceLabelsFromProject(project), namespace.Labels) {
		log.Info("Referenced Namespace does not belong to this Project, nothing to do for releasing it")
		return true, nil
	}

	// If the user wants to keep the namespace in the system even if the project gets deleted then we remove the related
	// labels, annotations, and owner references and only delete the project.
	var keepNamespace bool
	if val, ok := namespace.Annotations[v1beta1constants.NamespaceKeepAfterProjectDeletion]; ok {
		keepNamespace, _ = strconv.ParseBool(val)
	}

	if keepNamespace {
		delete(namespace.Annotations, v1beta1constants.NamespaceProject)
		delete(namespace.Annotations, v1beta1constants.NamespaceKeepAfterProjectDeletion)
		delete(namespace.Annotations, v1beta1constants.NamespaceCreatedByProjectController)
		delete(namespace.Labels, v1beta1constants.ProjectName)
		delete(namespace.Labels, v1beta1constants.GardenRole)
		for i := len(namespace.OwnerReferences) - 1; i >= 0; i-- {
			if ownerRef := namespace.OwnerReferences[i]; ownerRef.APIVersion == gardencorev1beta1.SchemeGroupVersion.String() &&
				ownerRef.Kind == "Project" &&
				ownerRef.Name == project.Name &&
				ownerRef.UID == project.UID {
				namespace.OwnerReferences = append(namespace.OwnerReferences[:i], namespace.OwnerReferences[i+1:]...)
			}
		}
		log.Info("Project Namespace should be kept, removing owner references")
		err := r.Client.Update(ctx, namespace)
		return true, err
	}

	log.Info("Deleting Project Namespace")
	err := r.Client.Delete(ctx, namespace, kubernetes.DefaultDeleteOptions...)
	return false, client.IgnoreNotFound(err)
}
