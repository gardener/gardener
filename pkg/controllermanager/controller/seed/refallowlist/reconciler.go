// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package refallowlist

import (
	"context"
	"fmt"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Secret/ConfigMap/WorkloadIdentity objects and ensures they carry the
// `seed.gardener.cloud/names` annotation listing every seed that references them.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.SeedReferenceAllowlistControllerConfiguration
}

// Reconcile recomputes the desired `seed.gardener.cloud/names` annotation for the given object
// by scanning all Gardenlet and ManagedSeed objects, then patches the object if needed.
func (r *Reconciler) Reconcile(ctx context.Context, req Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	obj := &metav1.PartialObjectMetadata{}
	obj.SetGroupVersionKind(groupVersionKindForRequest(req))
	obj.SetNamespace(req.Namespace)
	obj.SetName(req.Name)

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Referenced resource not found, skipping")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	desired, err := r.desiredSeedNames(ctx, req)
	if err != nil {
		return reconcile.Result{}, err
	}

	current := obj.GetAnnotations()[v1beta1constants.AnnotationSeedNames]
	if desired == current {
		return reconcile.Result{}, nil
	}

	log.Info("Updating annotation", "current", current, "desired", desired)

	patch := client.MergeFromWithOptions(obj.DeepCopyObject().(client.Object), client.MergeFromWithOptimisticLock{})
	if desired == "" {
		annotations := obj.GetAnnotations()
		delete(annotations, v1beta1constants.AnnotationSeedNames)
		obj.SetAnnotations(annotations)
	} else {
		kubernetesutils.SetMetaDataAnnotation(obj, v1beta1constants.AnnotationSeedNames, desired)
	}
	return reconcile.Result{}, r.Client.Patch(ctx, obj, patch)
}

// desiredSeedNames computes the desired annotation value for the given object by collecting all
// seed names from Gardenlet and ManagedSeed objects that reference it.
func (r *Reconciler) desiredSeedNames(ctx context.Context, req Request) (string, error) {
	seedNames := sets.New[string]()

	gardenletList := &seedmanagementv1alpha1.GardenletList{}
	if err := r.Client.List(ctx, gardenletList); err != nil {
		return "", fmt.Errorf("failed listing Gardenlets: %w", err)
	}
	for _, gardenlet := range gardenletList.Items {
		if slices.Contains(refsFromConfig(&gardenlet.Spec.Config), req) {
			seedNames.Insert(gardenlet.Name)
		}
	}

	managedSeedList := &seedmanagementv1alpha1.ManagedSeedList{}
	if err := r.Client.List(ctx, managedSeedList); err != nil {
		return "", fmt.Errorf("failed listing ManagedSeeds: %w", err)
	}
	for _, managedSeed := range managedSeedList.Items {
		if slices.Contains(refsFromConfig(&managedSeed.Spec.Gardenlet.Config), req) {
			seedNames.Insert(managedSeed.Name)
		}
	}

	return strings.Join(sets.List(seedNames), ","), nil
}

// refsFromConfig decodes the gardenlet configuration and returns the set of Requests
// for resources referenced in the embedded SeedSpec. Returns nil if there is no SeedConfig.
func refsFromConfig(config *runtime.RawExtension) []Request {
	gardenletConfig, err := encoding.DecodeGardenletConfiguration(config, false)
	if err != nil || gardenletConfig.SeedConfig == nil {
		return nil
	}
	return refsFromSeedSpec(&gardenletConfig.SeedConfig.Spec)
}

// groupVersionKindForRequest returns the GroupVersionKind for the given Request.
func groupVersionKindForRequest(req Request) schema.GroupVersionKind {
	gv, _ := schema.ParseGroupVersion(req.APIVersion)
	return gv.WithKind(req.Kind)
}

func refsFromSeedSpec(spec *gardencorev1beta1.SeedSpec) []Request {
	var refs []Request

	if spec.Backup != nil && spec.Backup.CredentialsRef != nil {
		c := spec.Backup.CredentialsRef
		refs = append(refs, Request{Namespace: c.Namespace, Name: c.Name, APIVersion: c.APIVersion, Kind: c.Kind})
	}

	if spec.DNS.Provider != nil && spec.DNS.Provider.CredentialsRef != nil {
		c := spec.DNS.Provider.CredentialsRef
		refs = append(refs, Request{Namespace: c.Namespace, Name: c.Name, APIVersion: c.APIVersion, Kind: c.Kind})
	}

	if spec.DNS.Internal != nil {
		c := spec.DNS.Internal.CredentialsRef
		refs = append(refs, Request{Namespace: c.Namespace, Name: c.Name, APIVersion: c.APIVersion, Kind: c.Kind})
	}

	for _, dns := range spec.DNS.Defaults {
		c := dns.CredentialsRef
		refs = append(refs, Request{Namespace: c.Namespace, Name: c.Name, APIVersion: c.APIVersion, Kind: c.Kind})
	}

	for _, resource := range spec.Resources {
		c := resource.ResourceRef
		refs = append(refs, Request{Namespace: v1beta1constants.GardenNamespace, Name: c.Name, APIVersion: c.APIVersion, Kind: c.Kind})
	}

	return refs
}
