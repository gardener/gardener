// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package refallowlist

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles referenced credential objects and ensures they carry the
// `seed.gardener.cloud/names` annotation listing every seed that references them.
type Reconciler struct {
	Client client.Client
}

// Reconcile reconciles a referenced credential object by adding or removing the seed name
// in the `seed.gardener.cloud/names` annotation based on the request.
func (r *Reconciler) Reconcile(ctx context.Context, req Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	obj := objectForRef(req.Namespace, req.Name, req.APIVersion, req.Kind)
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			log.V(1).Info("Referenced resource not found or type unknown, skipping", "apiVersion", req.APIVersion, "kind", req.Kind, "namespace", req.Namespace, "name", req.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	current := obj.GetAnnotations()[v1beta1constants.AnnotationSeedNames]
	var desired string
	if req.Remove {
		desired = removeFromCSV(current, req.SeedName)
	} else {
		desired = addToCSV(current, req.SeedName)
	}

	if desired == current {
		return reconcile.Result{}, nil
	}

	if req.Remove {
		log.Info("Removing seed from annotation", "seed", req.SeedName)
	} else {
		log.Info("Adding seed to annotation", "seed", req.SeedName)
	}

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

func objectForRef(namespace, name, apiVersion, kind string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}

type ref struct {
	namespace  string
	name       string
	apiVersion string
	kind       string
}

func refsFromSeedSpec(spec *gardencorev1beta1.SeedSpec) []ref {
	var refs []ref

	if spec.Backup != nil && spec.Backup.CredentialsRef != nil {
		c := spec.Backup.CredentialsRef
		refs = append(refs, ref{namespace: c.Namespace, name: c.Name, apiVersion: c.APIVersion, kind: c.Kind})
	}

	if spec.DNS.Provider != nil && spec.DNS.Provider.CredentialsRef != nil {
		c := spec.DNS.Provider.CredentialsRef
		refs = append(refs, ref{namespace: c.Namespace, name: c.Name, apiVersion: c.APIVersion, kind: c.Kind})
	}

	if spec.DNS.Internal != nil {
		c := spec.DNS.Internal.CredentialsRef
		refs = append(refs, ref{namespace: c.Namespace, name: c.Name, apiVersion: c.APIVersion, kind: c.Kind})
	}

	for _, dns := range spec.DNS.Defaults {
		c := dns.CredentialsRef
		refs = append(refs, ref{namespace: c.Namespace, name: c.Name, apiVersion: c.APIVersion, kind: c.Kind})
	}

	for _, resource := range spec.Resources {
		c := resource.ResourceRef
		refs = append(refs, ref{namespace: v1beta1constants.GardenNamespace, name: c.Name, apiVersion: c.APIVersion, kind: c.Kind})
	}

	return refs
}

func addToCSV(csv, value string) string {
	for token := range strings.SplitSeq(csv, ",") {
		if strings.TrimSpace(token) == value {
			return csv
		}
	}
	if csv == "" {
		return value
	}
	return csv + "," + value
}

func removeFromCSV(csv, value string) string {
	tokens := strings.Split(csv, ",")
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if t := strings.TrimSpace(token); t != "" && t != value {
			result = append(result, t)
		}
	}
	return strings.Join(result, ",")
}
