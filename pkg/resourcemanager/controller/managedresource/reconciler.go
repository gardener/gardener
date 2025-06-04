// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcesv1alpha1helper "github.com/gardener/gardener/pkg/apis/resources/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	utilclient "github.com/gardener/gardener/pkg/utils/kubernetes/client"
	managedresourcesutils "github.com/gardener/gardener/pkg/utils/managedresources"
)

var (
	deletePropagationForeground = metav1.DeletePropagationForeground
	foregroundDeletionAPIGroups = sets.New(appsv1.GroupName, extensionsv1beta1.GroupName, batchv1.GroupName)
)

// Reconciler manages the resources reference by ManagedResources.
type Reconciler struct {
	SourceClient                  client.Client
	TargetClient                  client.Client
	TargetScheme                  *runtime.Scheme
	TargetRESTMapper              meta.RESTMapper
	Config                        resourcemanagerconfigv1alpha1.ManagedResourceControllerConfig
	Clock                         clock.Clock
	ClassFilter                   *resourcemanagerpredicate.ClassFilter
	ClusterID                     string
	GarbageCollectorActivated     bool
	RequeueAfterOnDeletionPending *time.Duration
}

// Reconcile manages the resources reference by ManagedResources.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	mr := &resourcesv1alpha1.ManagedResource{}
	if err := r.SourceClient.Get(ctx, req.NamespacedName, mr); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if ignore(mr) && mr.DeletionTimestamp == nil {
		log.Info("Skipping reconciliation since ManagedResource is ignored")
		if err := r.updateConditionsForIgnoredManagedResource(ctx, mr); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}

		return reconcile.Result{}, nil
	}

	// If the object should be deleted or the responsibility changed
	// the actual deployments have to be deleted
	if isTransferringResponsibility := r.ClassFilter.IsTransferringResponsibility(mr); mr.DeletionTimestamp != nil || isTransferringResponsibility {
		if isTransferringResponsibility {
			log.Info("Class of ManagedResource changed. Cleaning resources as the responsibility changed")
		}
		return r.delete(ctx, log, mr)
	}

	// If the deletion after a change of responsibility is still
	// pending, the handling of the object by the responsible controller
	// must be delayed, until the deletion is finished.
	if r.ClassFilter.IsWaitForCleanupRequired(mr) {
		log.Info("Waiting for previous handler to clean resources created by ManagedResource")
		return reconcile.Result{Requeue: true}, nil
	}

	if err := managedresourcesutils.EnsureSigningKeys(ctx, r.SourceClient); err != nil {
		log.Error(err, "Could not ensure signing secret")
		return reconcile.Result{}, fmt.Errorf("could not ensure signing secret: %w", err)
	}

	return r.reconcile(ctx, log, mr)
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, mr *resourcesv1alpha1.ManagedResource) (reconcile.Result, error) {
	log.Info("Starting to reconcile ManagedResource")

	if !controllerutil.ContainsFinalizer(mr, r.ClassFilter.FinalizerName()) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.SourceClient, mr, r.ClassFilter.FinalizerName()); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	var (
		newResourcesObjects          []object
		newResourcesObjectReferences []resourcesv1alpha1.ObjectReference
		orphanedObjectReferences     []resourcesv1alpha1.ObjectReference

		equivalences           = NewEquivalences(mr.Spec.Equivalences...)
		existingResourcesIndex = NewObjectIndex(mr.Status.Resources, equivalences)
		origin                 = resourcesv1alpha1helper.OriginForManagedResource(r.ClusterID, mr)

		forceOverwriteLabels      bool
		forceOverwriteAnnotations bool

		decodingErrors []*decodingError

		hash = sha256.New()
	)

	if v := mr.Spec.ForceOverwriteLabels; v != nil {
		forceOverwriteLabels = *v
	}
	if v := mr.Spec.ForceOverwriteAnnotations; v != nil {
		forceOverwriteAnnotations = *v
	}

	reconcileCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	// Initialize condition based on the current status.
	conditionResourcesApplied := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)

	for _, ref := range mr.Spec.SecretRefs {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: mr.Namespace}}
		if err := r.SourceClient.Get(reconcileCtx, client.ObjectKeyFromObject(secret), secret); err != nil {
			conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, gardencorev1beta1.ConditionFalse, "CannotReadSecret", err.Error())
			if err := updateConditions(ctx, r.SourceClient, mr, conditionResourcesApplied); err != nil {
				return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
			}

			return reconcile.Result{}, fmt.Errorf("could not read secret '%s': %+v", secret.Name, err)
		}

		// Sort secret's data key to keep consistent ordering while calculating checksum
		secretKeys := make([]string, 0, len(secret.Data))
		for secretKey := range secret.Data {
			secretKeys = append(secretKeys, secretKey)
		}
		slices.Sort(secretKeys)

		err := managedresourcesutils.VerifySecretSignature(ctx, r.SourceClient, secret)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("could not verify signature of secret '%s': %w", secret.Name, err)
		}

		for _, secretKey := range secretKeys {
			var reader io.Reader = bytes.NewReader(secret.Data[secretKey])
			if strings.HasSuffix(secretKey, resourcesv1alpha1.BrotliCompressionSuffix) {
				reader = brotli.NewReader(reader)
			}

			var (
				decoder    = yaml.NewYAMLOrJSONDecoder(reader, 1024)
				decodedObj map[string]any
			)

			for indexInFile := 0; true; indexInFile++ {
				objLog := log.WithValues("secret", client.ObjectKeyFromObject(secret), "secretKey", secretKey, "indexInFile", indexInFile)

				err := decoder.Decode(&decodedObj)
				if err == io.EOF {
					break
				}
				if err != nil {
					dErr := &decodingError{
						err:         err,
						secret:      client.ObjectKeyFromObject(secret),
						secretKey:   secretKey,
						indexInFile: indexInFile,
					}
					decodingErrors = append(decodingErrors, dErr)
					objLog.Error(dErr.err, "Could not decode resource")
					continue
				}

				if decodedObj == nil {
					continue
				}

				obj := &unstructured.Unstructured{Object: decodedObj}
				objLog = objLog.WithValues("object", client.Object(obj))

				// look up scope of objects' kind to check, if we should default the namespace field
				mapping, err := r.TargetRESTMapper.RESTMapping(obj.GroupVersionKind().GroupKind(), obj.GroupVersionKind().Version)
				if err != nil || mapping == nil {
					// Cache miss most probably indicates, that the corresponding CRD is not yet applied.
					// CRD might be applied later as part of the ManagedResource reconciliation

					errMsg := "<nil>"
					if err != nil {
						errMsg = err.Error()
					}
					objLog.Info("Could not get RESTMapping for object", "err", errMsg)

					// default namespace on a best effort basis
					if obj.GetKind() != "Namespace" && obj.GetNamespace() == "" {
						obj.SetNamespace(metav1.NamespaceDefault)
					}
				} else {
					if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
						// default namespace field to `default` in case of namespaced kinds
						if obj.GetNamespace() == "" {
							obj.SetNamespace(metav1.NamespaceDefault)
						}
					} else {
						// unset namespace field in case of non-namespaced kinds
						obj.SetNamespace("")
					}
				}

				var (
					newObj = object{
						obj:                       obj,
						forceOverwriteLabels:      forceOverwriteLabels,
						forceOverwriteAnnotations: forceOverwriteAnnotations,
					}
					objectReference = resourcesv1alpha1.ObjectReference{
						ObjectReference: corev1.ObjectReference{
							APIVersion: newObj.obj.GetAPIVersion(),
							Kind:       newObj.obj.GetKind(),
							Name:       newObj.obj.GetName(),
							Namespace:  newObj.obj.GetNamespace(),
						},
						Labels:      mergeMaps(newObj.obj.GetLabels(), mr.Spec.InjectLabels),
						Annotations: newObj.obj.GetAnnotations(),
					}
				)

				objectReference.Labels[resourcesv1alpha1.ManagedBy] = *r.Config.ManagedByLabelValue

				var found bool
				newObj.oldInformation, found = existingResourcesIndex.Lookup(objectReference)
				decodedObj = nil

				if ignoreMode(obj) {
					if found {
						orphanedObjectReferences = append(orphanedObjectReferences, objectReference)
					}

					objLog.Info("Skipping object because it is marked to be ignored")
					continue
				}

				hash.Write(secret.Data[secretKey])
				newResourcesObjects = append(newResourcesObjects, newObj)
				newResourcesObjectReferences = append(newResourcesObjectReferences, objectReference)
			}
		}
	}

	// calculate the checksum for the referenced secrets data.
	secretsDataChecksum := hex.EncodeToString(hash.Sum(nil))

	// sort object references before updating status, to keep consistent ordering
	// (otherwise, the order will be different on each update)
	sortObjectReferences(newResourcesObjectReferences)

	// invalidate conditions, if resources have been added/removed from the managed resource
	if !apiequality.Semantic.DeepEqual(mr.Status.Resources, newResourcesObjectReferences) || mr.Status.SecretsDataChecksum == nil || *mr.Status.SecretsDataChecksum != secretsDataChecksum {
		conditionResourcesHealthy := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesHealthy)
		conditionResourcesHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesHealthy, gardencorev1beta1.ConditionUnknown,
			resourcesv1alpha1.ConditionChecksPending, "The health checks have not yet been executed for the current set of resources.")
		conditionResourcesProgressing := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesProgressing)
		conditionResourcesProgressing = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesProgressing, gardencorev1beta1.ConditionUnknown,
			resourcesv1alpha1.ConditionChecksPending, "Checks have not yet been executed for the current set of resources.")

		reason := resourcesv1alpha1.ConditionApplyProgressing
		msg := "The resources are currently being reconciled."
		switch conditionResourcesApplied.Reason {
		case resourcesv1alpha1.ConditionApplyFailed, resourcesv1alpha1.ConditionDeletionFailed, resourcesv1alpha1.ConditionDeletionPending:
			// keep condition reason and message if last reconciliation failed
			reason = conditionResourcesApplied.Reason
			msg = conditionResourcesApplied.Message
		}
		conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, gardencorev1beta1.ConditionProgressing, reason, msg)

		if err := updateConditions(ctx, r.SourceClient, mr, conditionResourcesHealthy, conditionResourcesProgressing, conditionResourcesApplied); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}
	}

	if deletionPending, err := r.cleanOldResources(reconcileCtx, log, mr, existingResourcesIndex); err != nil {
		var (
			reason string
			status gardencorev1beta1.ConditionStatus
		)
		if deletionPending {
			reason = resourcesv1alpha1.ConditionDeletionPending
			status = gardencorev1beta1.ConditionProgressing
			log.Info("Deletion is still pending", "err", err)
		} else {
			reason = resourcesv1alpha1.ConditionDeletionFailed
			status = gardencorev1beta1.ConditionFalse
			log.Error(err, "Deletion of old resources failed")
		}

		conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, status, reason, err.Error())
		if err := updateConditions(ctx, r.SourceClient, mr, conditionResourcesApplied); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}

		if deletionPending {
			return reconcile.Result{RequeueAfter: *r.RequeueAfterOnDeletionPending}, nil
		} else {
			return reconcile.Result{}, err
		}
	}

	if err := r.releaseOrphanedResources(ctx, log, orphanedObjectReferences, origin); err != nil {
		conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, gardencorev1beta1.ConditionFalse, resourcesv1alpha1.ReleaseOfOrphanedResourcesFailed, err.Error())
		if err := updateConditions(ctx, r.SourceClient, mr, conditionResourcesApplied); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}

		return reconcile.Result{}, fmt.Errorf("could not release all orphaned resources: %+v", err)
	}

	injectLabels := mergeMaps(mr.Spec.InjectLabels, map[string]string{resourcesv1alpha1.ManagedBy: *r.Config.ManagedByLabelValue})
	if err := r.applyNewResources(reconcileCtx, log, origin, newResourcesObjects, injectLabels, equivalences); err != nil {
		conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, gardencorev1beta1.ConditionFalse, resourcesv1alpha1.ConditionApplyFailed, err.Error())
		if err := updateConditions(ctx, r.SourceClient, mr, conditionResourcesApplied); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}

		return reconcile.Result{}, fmt.Errorf("could not apply all new resources: %+v", err)
	}

	if len(decodingErrors) != 0 {
		conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, gardencorev1beta1.ConditionFalse, resourcesv1alpha1.ConditionDecodingFailed, fmt.Sprintf("Could not decode all new resources: %v", decodingErrors))
	} else {
		conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, gardencorev1beta1.ConditionTrue, resourcesv1alpha1.ConditionApplySucceeded, "All resources are applied.")
	}

	if err := updateManagedResourceStatus(ctx, r.SourceClient, mr, &secretsDataChecksum, newResourcesObjectReferences, conditionResourcesApplied); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
	}

	log.Info("Finished to reconcile ManagedResource")
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) delete(ctx context.Context, log logr.Logger, mr *resourcesv1alpha1.ManagedResource) (reconcile.Result, error) {
	log.Info("Started deleting resources created by ManagedResource")

	deleteCtx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	if err := r.updateConditionsForDeletion(ctx, mr); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
	}

	conditionResourcesApplied := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)

	if keepObjects := mr.Spec.KeepObjects; keepObjects == nil || !*keepObjects {
		existingResourcesIndex := NewObjectIndex(mr.Status.Resources, nil)

		msg := "The resources are currently being deleted."
		switch conditionResourcesApplied.Reason {
		case resourcesv1alpha1.ConditionDeletionPending, resourcesv1alpha1.ConditionDeletionFailed:
			// keep condition message if deletion is pending / failed
			msg = conditionResourcesApplied.Message
		}
		conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, gardencorev1beta1.ConditionProgressing, resourcesv1alpha1.ConditionDeletionPending, msg)
		if err := updateConditions(ctx, r.SourceClient, mr, conditionResourcesApplied); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
		}

		if deletionPending, err := r.cleanOldResources(deleteCtx, log, mr, existingResourcesIndex); err != nil {
			var (
				reason string
				status gardencorev1beta1.ConditionStatus
			)
			if deletionPending {
				reason = resourcesv1alpha1.ConditionDeletionPending
				status = gardencorev1beta1.ConditionProgressing
				log.Info("Deletion is still pending", "err", err)
			} else {
				reason = resourcesv1alpha1.ConditionDeletionFailed
				status = gardencorev1beta1.ConditionFalse
				log.Error(err, "Deletion of all resources failed")
			}

			conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, status, reason, err.Error())
			if err := updateConditions(ctx, r.SourceClient, mr, conditionResourcesApplied); err != nil {
				return reconcile.Result{}, fmt.Errorf("could not update the ManagedResource status: %w", err)
			}

			if deletionPending {
				return reconcile.Result{RequeueAfter: *r.RequeueAfterOnDeletionPending}, nil
			} else {
				return reconcile.Result{}, err
			}
		}
	} else {
		log.Info("Skipping deletion of objects as ManagedResource is marked to keep objects")
	}

	log.Info("All resources have been deleted")

	if controllerutil.ContainsFinalizer(mr, r.ClassFilter.FinalizerName()) {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.SourceClient, mr, r.ClassFilter.FinalizerName()); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}

	log.Info("Finished deleting resources created by ManagedResource")
	return reconcile.Result{}, nil
}

func (r *Reconciler) updateConditionsForIgnoredManagedResource(ctx context.Context, mr *resourcesv1alpha1.ManagedResource) error {
	message := "ManagedResource is marked to be ignored."
	conditionResourcesApplied := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesApplied)
	conditionResourcesApplied = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesApplied, gardencorev1beta1.ConditionTrue, resourcesv1alpha1.ConditionManagedResourceIgnored, message)
	conditionResourcesHealthy := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesHealthy)
	conditionResourcesHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesHealthy, gardencorev1beta1.ConditionTrue, resourcesv1alpha1.ConditionManagedResourceIgnored, message)
	conditionResourcesProgressing := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesProgressing)
	conditionResourcesProgressing = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesProgressing, gardencorev1beta1.ConditionFalse, resourcesv1alpha1.ConditionManagedResourceIgnored, message)

	oldMr := mr.DeepCopy()
	mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, conditionResourcesApplied, conditionResourcesHealthy, conditionResourcesProgressing)
	if !apiequality.Semantic.DeepEqual(oldMr.Status.Conditions, mr.Status.Conditions) {
		return r.SourceClient.Status().Update(ctx, mr)
	}

	return nil
}

func (r *Reconciler) updateConditionsForDeletion(ctx context.Context, mr *resourcesv1alpha1.ManagedResource) error {
	conditionResourcesHealthy := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesHealthy)
	conditionResourcesHealthy = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesHealthy, gardencorev1beta1.ConditionFalse, resourcesv1alpha1.ConditionDeletionPending, "The resources are currently being deleted.")
	conditionResourcesProgressing := v1beta1helper.GetOrInitConditionWithClock(r.Clock, mr.Status.Conditions, resourcesv1alpha1.ResourcesProgressing)
	conditionResourcesProgressing = v1beta1helper.UpdatedConditionWithClock(r.Clock, conditionResourcesProgressing, gardencorev1beta1.ConditionTrue, resourcesv1alpha1.ConditionDeletionPending, "The resources are currently being deleted.")
	return updateConditions(ctx, r.SourceClient, mr, conditionResourcesHealthy, conditionResourcesProgressing)
}

func (r *Reconciler) applyNewResources(ctx context.Context, log logr.Logger, origin string, newResourcesObjects []object, labelsToInject map[string]string, equivalences Equivalences) error {
	newResourcesObjects = sortByKind(newResourcesObjects)

	// get all HPA targetRefs to check if we should prevent overwriting replicas.
	// VPAs don't have to be checked, as they don't update the spec directly and only mutate Pods via a MutatingWebhook
	// and therefore don't interfere with the resource manager.
	horizontallyScaledObjects, err := computeHorizontallyScaledObjectKeys(ctx, r.TargetClient)
	if err != nil {
		return fmt.Errorf("failed to compute all HPA target ref object keys: %w", err)
	}

	for _, obj := range newResourcesObjects {
		var (
			current            = obj.obj.DeepCopy()
			resource           = unstructuredToString(obj.obj)
			scaledHorizontally = isScaled(obj.obj, horizontallyScaledObjects, equivalences)
		)

		resourceLogger := log.WithValues("resource", resource)

		resourceLogger.V(1).Info("Applying")

		operationResult, err := controllerutils.TypedCreateOrUpdate(ctx, r.TargetClient, r.TargetScheme, current, ptr.Deref(r.Config.AlwaysUpdate, false), func() error {
			metadata, err := meta.Accessor(obj.obj)
			if err != nil {
				return fmt.Errorf("error getting metadata of object %q: %s", resource, err)
			}

			// if the ignore annotation is set to false, do nothing (ignore the resource)
			if ignore(metadata) {
				annotations := current.GetAnnotations()
				delete(annotations, descriptionAnnotation)
				current.SetAnnotations(annotations)
				return nil
			}

			if err := injectLabels(obj.obj, labelsToInject); err != nil {
				return fmt.Errorf("error injecting labels into object %q: %s", resource, err)
			}

			return merge(origin, obj.obj, current, obj.forceOverwriteLabels, obj.oldInformation.Labels, obj.forceOverwriteAnnotations, obj.oldInformation.Annotations, scaledHorizontally)
		})
		if err != nil {
			if apierrors.IsConflict(err) {
				return err
			}

			if apierrors.IsInvalid(err) && operationResult == controllerutil.OperationResultUpdated && deleteOnInvalidUpdate(current, err) {
				if deleteErr := r.TargetClient.Delete(ctx, current); client.IgnoreNotFound(deleteErr) != nil {
					return fmt.Errorf("error deleting object %q after 'invalid' update error: %s", resource, deleteErr)
				}
				// return error directly, so that the create after delete will be retried
				return fmt.Errorf("deleted object %q because of 'invalid' update error, and 'delete-on-invalid-update' annotation on object or the resource is an immutable ConfigMap/Secret: %s", resource, err)
			}

			return fmt.Errorf("error during apply of object %q: %s", resource, err)
		}

		switch operationResult {
		case controllerutil.OperationResultCreated:
			resourceLogger.Info("Created resource because it was not existing before")
		case controllerutil.OperationResultUpdated:
			resourceLogger.Info("Updated resource because its actual state differed from the desired state")
		case controllerutil.OperationResultNone:
			resourceLogger.V(1).Info("Resource was neither created nor updated because its actual state matches with the desired state")
		}
	}

	return nil
}

// computeHorizontallyScaledObjectKeys returns a set of object keys (in the form `Group/Kind/Namespace/Name`)
// to objects that are horizontally scaled by HPA.
// VPAs are not checked, as they don't update the spec of Deployments/StatefulSets/... and only mutate resource
// requirements via a MutatingWebhook. This way VPAs don't interfere with the resource manager and must not be considered.
func computeHorizontallyScaledObjectKeys(ctx context.Context, c client.Client) (horizontallyScaledObjects sets.Set[string], err error) {
	horizontallyScaledObjects = sets.New[string]()

	// get all HPAs' targets
	hpaList := &autoscalingv1.HorizontalPodAutoscalerList{}
	if err := c.List(ctx, hpaList); err != nil && !meta.IsNoMatchError(err) {
		return horizontallyScaledObjects, fmt.Errorf("failed to list all HPAs: %w", err)
	}

	for _, hpa := range hpaList.Items {
		if key, err := targetObjectKeyFromHPA(hpa); err != nil {
			return horizontallyScaledObjects, err
		} else {
			horizontallyScaledObjects.Insert(key)
		}
	}

	return horizontallyScaledObjects, nil
}

func targetObjectKeyFromHPA(hpa autoscalingv1.HorizontalPodAutoscaler) (string, error) {
	targetGV, err := schema.ParseGroupVersion(hpa.Spec.ScaleTargetRef.APIVersion)
	if err != nil {
		return "", fmt.Errorf("invalid API version in scaleTargetReference of HorizontalPodAutoscaler '%s/%s': %w", hpa.Namespace, hpa.Name, err)
	}

	return objectKey(targetGV.Group, hpa.Spec.ScaleTargetRef.Kind, hpa.Namespace, hpa.Spec.ScaleTargetRef.Name), nil
}

func isScaled(obj *unstructured.Unstructured, scaledObjectKeys sets.Set[string], equivalences Equivalences) bool {
	key := objectKeyFromUnstructured(obj)

	if scaledObjectKeys.Has(key) {
		return true
	}

	// check if a HPA targets this object via an equivalent API Group
	gk := metav1.GroupKind{
		Group: obj.GroupVersionKind().Group,
		Kind:  obj.GetKind(),
	}
	for equivalentGroupKind := range equivalences.GetEquivalencesFor(gk) {
		if scaledObjectKeys.Has(objectKey(equivalentGroupKind.Group, equivalentGroupKind.Kind, obj.GetNamespace(), obj.GetName())) {
			return true
		}
	}

	return false
}

func objectKeyFromUnstructured(o *unstructured.Unstructured) string {
	return objectKey(o.GroupVersionKind().Group, o.GetKind(), o.GetNamespace(), o.GetName())
}

func ignoreMode(meta metav1.Object) bool {
	annotations := meta.GetAnnotations()
	return annotations[resourcesv1alpha1.Mode] == resourcesv1alpha1.ModeIgnore
}

func ignore(meta metav1.Object) bool {
	return keyExistsAndValueTrue(meta.GetAnnotations(), resourcesv1alpha1.Ignore)
}

func deleteOnInvalidUpdate(obj *unstructured.Unstructured, err error) bool {
	isImmutableConfigMapOrSecret := false
	if obj.GetAPIVersion() == "v1" && sets.New("ConfigMap", "Secret").Has(obj.GetKind()) {
		cause, ok := apierrors.StatusCause(err, metav1.CauseType(field.ErrorTypeForbidden))
		if ok && strings.Contains(cause.Message, "field is immutable when `immutable` is set") {
			isImmutableConfigMapOrSecret = true
		}
	}

	return keyExistsAndValueTrue(obj.GetAnnotations(), resourcesv1alpha1.DeleteOnInvalidUpdate) || isImmutableConfigMapOrSecret
}

func keepObject(meta metav1.Object) bool {
	return keyExistsAndValueTrue(meta.GetAnnotations(), resourcesv1alpha1.KeepObject)
}

func isGarbageCollectableResource(obj *unstructured.Unstructured) bool {
	return keyExistsAndValueTrue(obj.GetLabels(), references.LabelKeyGarbageCollectable) &&
		obj.GetAPIVersion() == "v1" && sets.New("ConfigMap", "Secret").Has(obj.GetKind())
}

func keyExistsAndValueTrue(kv map[string]string, key string) bool {
	if kv == nil {
		return false
	}
	val, exists := kv[key]
	valueTrue, _ := strconv.ParseBool(val)
	return exists && valueTrue
}

func (r *Reconciler) cleanOldResources(ctx context.Context, log logr.Logger, mr *resourcesv1alpha1.ManagedResource, index *objectIndex) (bool, error) {
	type output struct {
		obj             *unstructured.Unstructured
		deletionPending bool
		err             error
	}

	var (
		results         = make(chan *output)
		wg              sync.WaitGroup
		deletePVCs      = mr.Spec.DeletePersistentVolumeClaims != nil && *mr.Spec.DeletePersistentVolumeClaims
		deletionPending = false
		errorList       = &multierror.Error{
			ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("Could not clean all old resources"),
		}
	)

	for _, oldResource := range index.Objects() {
		if !index.Found(oldResource) {
			wg.Add(1)
			go func(ref resourcesv1alpha1.ObjectReference) {
				defer wg.Done()

				obj := &unstructured.Unstructured{}
				obj.SetAPIVersion(ref.APIVersion)
				obj.SetKind(ref.Kind)
				obj.SetNamespace(ref.Namespace)
				obj.SetName(ref.Name)

				logger := log.WithValues("resource", unstructuredToString(obj))
				logger.Info("Deleting")

				// get object before deleting to be able to do cleanup work for it
				if err := r.TargetClient.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, obj); err != nil {
					if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
						logger.Error(err, "Error during deletion")
						results <- &output{obj, true, err}
						return
					}

					// resource already deleted, nothing to do here
					results <- &output{obj, false, nil}
					return
				}

				if keepObject(obj) {
					logger.Info("Keeping object in the system as "+resourcesv1alpha1.KeepObject+" annotation found", "resource", unstructuredToString(obj))
					results <- &output{obj, false, nil}
					return
				}

				if r.GarbageCollectorActivated && isGarbageCollectableResource(obj) {
					logger.Info("Keeping object in the system as it is marked as 'garbage-collectable'", "resource", unstructuredToString(obj))
					results <- &output{obj, false, nil}
					return
				}

				if err := cleanup(ctx, r.TargetClient, r.TargetScheme, obj, deletePVCs); err != nil {
					logger.Error(err, "Error during cleanup")
					results <- &output{obj, true, err}
					return
				}

				deleteOptions := &client.DeleteOptions{}

				// only delete resources in specific API groups with foreground deletion propagation
				// see https://github.com/kubernetes/kubernetes/issues/91621, https://github.com/kubernetes/kubernetes/issues/91287
				// and similar, because of which some objects (e.g `rbac/*` or `v1/Service`) cannot be deleted reliably
				// with foreground deletion propagation.
				if foregroundDeletionAPIGroups.Has(obj.GroupVersionKind().Group) {
					// delete with DeletePropagationForeground to be sure to cleanup all resources (e.g. batch/v1beta1.CronJob
					// defaults PropagationPolicy to Orphan for backwards compatibility, so it will orphan its Jobs)
					deleteOptions.PropagationPolicy = &deletePropagationForeground
				}

				if err := r.TargetClient.Delete(ctx, obj, deleteOptions); err != nil {
					if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
						logger.Error(err, "Error during deletion")
						results <- &output{obj, true, err}
						return
					}
					results <- &output{obj, false, nil}
					return
				}

				if err := finalizeResourceIfNecessary(ctx, logger, r.TargetClient, r.Clock, obj); err != nil {
					logger.Error(err, "Error when finalizing resource if necessary")
					results <- &output{obj, true, err}
					return
				}

				results <- &output{obj, true, nil}
			}(oldResource)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for out := range results {
		resource := unstructuredToString(out.obj)
		if out.deletionPending {
			deletionPending = true
			errMsg := fmt.Sprintf("deletion of old resource %q is still pending", resource)
			if out.err != nil {
				errMsg = fmt.Sprintf("%s: %v", errMsg, out.err)
			}

			// consult service events for more details
			eventsMsg, err := eventsForObject(ctx, r.TargetScheme, r.TargetClient, out.obj)
			if err != nil {
				log.Error(err, "Error reading events for more information", "resource", resource)
			} else if eventsMsg != "" {
				errMsg = fmt.Sprintf("%s\n\n%s", errMsg, eventsMsg)
			}

			errorList = multierror.Append(errorList, errors.New(errMsg))
			continue
		}

		if out.err != nil {
			errorList = multierror.Append(errorList, fmt.Errorf("error during deletion of old resource %q: %w", resource, out.err))
		}
	}

	return deletionPending, errorList.ErrorOrNil()
}

func (r *Reconciler) releaseOrphanedResources(ctx context.Context, log logr.Logger, orphanedResources []resourcesv1alpha1.ObjectReference, origin string) error {
	var (
		results   = make(chan error)
		wg        sync.WaitGroup
		errorList = &multierror.Error{
			ErrorFormat: errorsutils.NewErrorFormatFuncWithPrefix("Could not release all orphaned resources"),
		}
	)

	for _, orphanedResource := range orphanedResources {
		wg.Add(1)

		go func(ref resourcesv1alpha1.ObjectReference) {
			defer wg.Done()

			err := r.releaseOrphanedResource(ctx, log, ref, origin)
			results <- err
		}(orphanedResource)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for err := range results {
		if err != nil {
			errorList = multierror.Append(errorList, err)
		}
	}

	return errorList.ErrorOrNil()
}

func (r *Reconciler) releaseOrphanedResource(ctx context.Context, log logr.Logger, ref resourcesv1alpha1.ObjectReference, origin string) error {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(ref.APIVersion)
	obj.SetKind(ref.Kind)
	obj.SetNamespace(ref.Namespace)
	obj.SetName(ref.Name)

	resource := unstructuredToString(obj)

	log.Info("Releasing orphan resource", "resource", resource)

	if err := r.TargetClient.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, obj); err != nil {
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return fmt.Errorf("error getting object %q: %w", resource, err)
		}

		return nil
	}

	// Skip the release of resource when the origin annotation has already changed
	objOrigin := obj.GetAnnotations()[resourcesv1alpha1.OriginAnnotation]
	if objOrigin != origin {
		log.Info("Skipping release for orphan resource as origin annotation has already changed", "resource", resource)
		return nil
	}

	oldObj := obj.DeepCopy()
	annotations := obj.GetAnnotations()
	delete(annotations, resourcesv1alpha1.OriginAnnotation)
	delete(annotations, descriptionAnnotation)
	obj.SetAnnotations(annotations)

	if err := r.TargetClient.Patch(ctx, obj, client.MergeFrom(oldObj)); err != nil {
		if !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return fmt.Errorf("error patching object %q: %w", resource, err)
		}

		return nil
	}

	return nil
}

func eventsForObject(ctx context.Context, scheme *runtime.Scheme, c client.Client, obj *unstructured.Unstructured) (string, error) {
	var (
		relevantGKs = []schema.GroupKind{
			corev1.SchemeGroupVersion.WithKind("Service").GroupKind(),
		}
		eventLimit = 5
	)

	for _, gk := range relevantGKs {
		if gk == obj.GetObjectKind().GroupVersionKind().GroupKind() {
			return kubernetesutils.FetchEventMessages(ctx, scheme, c, obj, corev1.EventTypeWarning, eventLimit)
		}
	}
	return "", nil
}

func updateManagedResourceStatus(
	ctx context.Context,
	c client.Client,
	mr *resourcesv1alpha1.ManagedResource,
	secretsDataChecksum *string,
	resources []resourcesv1alpha1.ObjectReference,
	updatedConditions ...gardencorev1beta1.Condition,
) error {
	mr.Status.Conditions = v1beta1helper.MergeConditions(mr.Status.Conditions, updatedConditions...)
	mr.Status.SecretsDataChecksum = secretsDataChecksum
	mr.Status.Resources = resources
	mr.Status.ObservedGeneration = mr.Generation
	return c.Status().Update(ctx, mr)
}

func updateConditions(ctx context.Context, c client.Client, mr *resourcesv1alpha1.ManagedResource, conditions ...gardencorev1beta1.Condition) error {
	newConditions := v1beta1helper.MergeConditions(mr.Status.Conditions, conditions...)
	mr.Status.Conditions = newConditions
	return c.Status().Update(ctx, mr)
}

func unstructuredToString(u *unstructured.Unstructured) string {
	// return no key, but an description including the version
	apiVersion, kind := u.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return objectKey(apiVersion, kind, u.GetNamespace(), u.GetName())
}

// injectLabels injects the given labels into the given object's metadata and if present also into the
// pod template's and volume claims templates' metadata
func injectLabels(obj *unstructured.Unstructured, labels map[string]string) error {
	if len(labels) == 0 {
		return nil
	}
	obj.SetLabels(mergeMaps(obj.GetLabels(), labels))

	if err := injectLabelsIntoPodTemplate(obj, labels); err != nil {
		return err
	}

	return injectLabelsIntoVolumeClaimTemplate(obj, labels)
}

func injectLabelsIntoPodTemplate(obj *unstructured.Unstructured, labels map[string]string) error {
	_, found, err := unstructured.NestedMap(obj.Object, "spec", "template")
	if err != nil {
		return err
	}
	if !found {
		return nil
	}

	templateLabels, _, err := unstructured.NestedStringMap(obj.Object, "spec", "template", "metadata", "labels")
	if err != nil {
		return err
	}

	return unstructured.SetNestedField(obj.Object, mergeLabels(templateLabels, labels), "spec", "template", "metadata", "labels")
}

func injectLabelsIntoVolumeClaimTemplate(obj *unstructured.Unstructured, labels map[string]string) error {
	volumeClaimTemplates, templatesFound, err := unstructured.NestedSlice(obj.Object, "spec", "volumeClaimTemplates")
	if err != nil {
		return err
	}
	if !templatesFound {
		return nil
	}

	for i, t := range volumeClaimTemplates {
		template, ok := t.(map[string]any)
		if !ok {
			return fmt.Errorf("failed to inject labels into .spec.volumeClaimTemplates[%d], is not a map[string]any", i)
		}

		templateLabels, _, err := unstructured.NestedStringMap(template, "metadata", "labels")
		if err != nil {
			return err
		}

		if err := unstructured.SetNestedField(template, mergeLabels(templateLabels, labels), "metadata", "labels"); err != nil {
			return err
		}
	}

	return unstructured.SetNestedSlice(obj.Object, volumeClaimTemplates, "spec", "volumeClaimTemplates")
}

func mergeLabels(existingLabels, newLabels map[string]string) map[string]any {
	if existingLabels == nil {
		return stringMapToInterfaceMap(newLabels)
	}

	labels := make(map[string]any, len(existingLabels)+len(newLabels))
	for k, v := range existingLabels {
		labels[k] = v
	}
	for k, v := range newLabels {
		labels[k] = v
	}
	return labels
}

func stringMapToInterfaceMap(in map[string]string) map[string]any {
	m := make(map[string]any, len(in))
	for k, v := range in {
		m[k] = v
	}
	return m
}

// mergeMaps merges the two string maps. If a key is present in both maps, the value in the second map takes precedence
func mergeMaps(one, two map[string]string) map[string]string {
	out := make(map[string]string, len(one)+len(two))
	for k, v := range one {
		out[k] = v
	}
	for k, v := range two {
		out[k] = v
	}
	return out
}

type object struct {
	obj                       *unstructured.Unstructured
	oldInformation            resourcesv1alpha1.ObjectReference
	forceOverwriteLabels      bool
	forceOverwriteAnnotations bool
}

type decodingError struct {
	err         error
	secret      client.ObjectKey
	secretKey   string
	indexInFile int
}

func (d *decodingError) StringShort() string {
	return fmt.Sprintf("Could not decode resource at index %d in '%s' in secret '%s'", d.indexInFile, d.secretKey, d.secret.String())
}

func (d *decodingError) String() string {
	return fmt.Sprintf("%s: %s.", d.StringShort(), d.err)
}

func finalizeResourceIfNecessary(ctx context.Context, log logr.Logger, cl client.Client, clock clock.Clock, obj *unstructured.Unstructured) error {
	var finalizeDeletionAfter time.Duration
	if v, ok := obj.GetAnnotations()[resourcesv1alpha1.FinalizeDeletionAfter]; ok {
		var err error
		finalizeDeletionAfter, err = time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("failed parsing duration of resources.gardener.cloud/finalize-deletion-after annotation: %w", err)
		}
	}

	if finalizeDeletionAfter == 0 {
		return nil
	}

	log.Info("Found resources.gardener.cloud/finalize-deletion-after annotation", "gracePeriod", finalizeDeletionAfter)

	finalizer := utilclient.NewFinalizer()
	if obj.GetAPIVersion() == "v1" && obj.GetKind() == "Namespace" {
		finalizer = utilclient.NewNamespaceFinalizer()
	}

	if obj.GetDeletionTimestamp() == nil || obj.GetDeletionTimestamp().Time.UTC().Add(finalizeDeletionAfter).After(clock.Now().UTC()) {
		log.Info("Cannot finalize resource yet since grace period has not yet elapsed", "deletionTimestamp", obj.GetDeletionTimestamp(), "gracePeriod", finalizeDeletionAfter)
		return nil
	}

	if hasFinalizers, err := finalizer.HasFinalizers(obj); err != nil {
		return fmt.Errorf("failed checking whether resource has finalizers: %w", err)
	} else if !hasFinalizers {
		return nil
	}

	log.Info("Removing finalizers from resource since grace period has elapsed", "deletionTimestamp", obj.GetDeletionTimestamp(), "gracePeriod", finalizeDeletionAfter)
	return finalizer.Finalize(ctx, cl, obj)
}
