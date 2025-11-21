// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/utils/flow"
	unstructuredutils "github.com/gardener/gardener/pkg/utils/kubernetes/unstructured"
	"github.com/gardener/gardener/pkg/utils/workloadidentity"
)

// labelWorkloadIdentityReferencedSecretKey is a label key used to indicate that the secret is referenced
// by a resource reference in a Shoot/Seed specification.
const labelWorkloadIdentityReferencedSecretKey = securityv1alpha1constants.WorkloadIdentityPrefix + "referenced"

// referencedWorkloadIdentitySecretLabels are the labels set on workload identity secrets created for referenced resources.
var referencedWorkloadIdentitySecretLabels = map[string]string{labelWorkloadIdentityReferencedSecretKey: "true"}

// PrepareReferencedResourcesForSeedCopy reads referenced objects prepares them for deployment to the seed cluster.
// Only resources of kind Secret and ConfigMap are considered.
func PrepareReferencedResourcesForSeedCopy(ctx context.Context, cl client.Client, resources []gardencorev1beta1.NamedResourceReference, sourceNamespace, targetNamespace string) ([]*unstructured.Unstructured, error) {
	var unstructuredObjs []*unstructured.Unstructured

	for _, resource := range resources {
		if resource.ResourceRef.APIVersion != corev1.SchemeGroupVersion.String() || !slices.Contains([]string{"Secret", "ConfigMap"}, resource.ResourceRef.Kind) {
			continue
		}

		// Read the resource from the Garden cluster
		obj, err := unstructuredutils.GetObjectByRef(ctx, cl, &resource.ResourceRef, sourceNamespace)
		if err != nil {
			return nil, err
		}
		if obj == nil {
			return nil, fmt.Errorf("object not found %v", resource.ResourceRef)
		}

		obj = unstructuredutils.FilterMetadata(obj, "finalizers")

		// Create an unstructured object and append it to the slice
		unstructuredObj := &unstructured.Unstructured{Object: obj}
		unstructuredObj.SetNamespace(targetNamespace)
		unstructuredObj.SetName(v1beta1constants.ReferencedResourcesPrefix + unstructuredObj.GetName())

		// We don't want to keep user-defined annotations or labels when copying the resource to the seed.
		unstructuredObj.SetAnnotations(nil)
		unstructuredObj.SetLabels(nil)

		unstructuredObjs = append(unstructuredObjs, unstructuredObj)
	}

	return unstructuredObjs, nil
}

// ReconcileWorkloadIdentityReferencedResources creates workload identity Secrets in the target namespace for every WorkloadIdentity reference.
// It also cleans up unreferenced workload identity Secrets in the target namespace.
// The secrets are named <[v1beta1constants.ReferencedWorkloadIdentityPrefix]> + <workload identity name>.
func ReconcileWorkloadIdentityReferencedResources(ctx context.Context, gardenClient, seedClient client.Client, resources []gardencorev1beta1.NamedResourceReference, sourceNamespace, targetNamespace string, referringObj client.Object) error {
	var (
		tasks           = []flow.TaskFn{}
		secretsToRetain = sets.New[string]()
	)

	for _, resource := range resources {
		if resource.ResourceRef.APIVersion != securityv1alpha1.SchemeGroupVersion.String() || resource.ResourceRef.Kind != "WorkloadIdentity" {
			continue
		}

		tasks = append(tasks, func(ctx context.Context) error {
			return createSecretForWorkloadIdentity(ctx, gardenClient, seedClient, resource.ResourceRef.Name, sourceNamespace, targetNamespace, referringObj, secretsToRetain)
		})
	}

	if err := flow.Parallel(tasks...)(ctx); err != nil {
		return fmt.Errorf("failed to deploy referenced workload identity secrets: %w", err)
	}

	if err := deleteUnreferencedWorkloadIdentitySecrets(ctx, seedClient, targetNamespace, secretsToRetain); err != nil {
		return fmt.Errorf("failed to delete unreferenced workload identity secrets: %w", err)
	}

	return nil
}

// createSecretForWorkloadIdentity creates a Secret in the target namespace in the seed for the given WorkloadIdentity.
// The secret name is added to the secretsToRetain parameter.
func createSecretForWorkloadIdentity(ctx context.Context, gardenClient, seedClient client.Client, workloadIdentityName, sourceNamespace, targetNamespace string, referringObj client.Object, secretsToRetain sets.Set[string]) error {
	gvk, err := gardenClient.GroupVersionKindFor(referringObj)
	if err != nil {
		return fmt.Errorf("failed to parse the GVK of the referring object: %w", err)
	}

	contextObject := securityv1alpha1.ContextObject{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       referringObj.GetName(),
		UID:        referringObj.GetUID(),
	}
	if ns := referringObj.GetNamespace(); ns != "" {
		contextObject.Namespace = &ns
	}

	secretName := v1beta1constants.ReferencedWorkloadIdentityPrefix + workloadIdentityName
	secretsToRetain.Insert(secretName)

	workloadIdentity := &securityv1alpha1.WorkloadIdentity{}
	if err := gardenClient.Get(ctx, client.ObjectKey{Namespace: sourceNamespace, Name: workloadIdentityName}, workloadIdentity); err != nil {
		return fmt.Errorf("failed to get WorkloadIdentity: %w", err)
	}

	s, err := workloadidentity.NewSecret(
		secretName,
		targetNamespace,
		workloadidentity.For(workloadIdentityName, sourceNamespace, workloadIdentity.Spec.TargetSystem.Type),
		workloadidentity.WithProviderConfig(workloadIdentity.Spec.TargetSystem.ProviderConfig),
		workloadidentity.WithContextObject(contextObject),
		workloadidentity.WithLabels(referencedWorkloadIdentitySecretLabels),
	)
	if err != nil {
		return fmt.Errorf("failed to create workload identity secret %s/%s: %w", targetNamespace, secretName, err)
	}
	if err := s.Reconcile(ctx, seedClient); err != nil {
		return fmt.Errorf("failed to reconcile workload identity secret %s/%s: %w", targetNamespace, secretName, err)
	}

	return nil
}

func deleteUnreferencedWorkloadIdentitySecrets(ctx context.Context, c client.Client, namespace string, secretsToRetain sets.Set[string]) error {
	secrets := &corev1.SecretList{}
	if err := c.List(ctx, secrets,
		client.InNamespace(namespace),
		client.MatchingLabels(referencedWorkloadIdentitySecretLabels),
	); err != nil {
		return fmt.Errorf("failed to list referenced workload identity secrets in namespace %s: %w", namespace, err)
	}

	tasks := []flow.TaskFn{}
	for _, secret := range secrets.Items {
		if secretsToRetain.Has(secret.Name) {
			continue
		}
		if !strings.HasPrefix(secret.Name, v1beta1constants.ReferencedWorkloadIdentityPrefix) {
			continue
		}

		tasks = append(tasks, func(ctx context.Context) error {
			return c.Delete(ctx, &secret)
		})
	}

	return flow.Parallel(tasks...)(ctx)
}

// DestroyWorkloadIdentityReferencedResources deletes the referenced workload identity Secrets in the target namespace.
func DestroyWorkloadIdentityReferencedResources(ctx context.Context, seedClient client.Client, targetNamespace string) error {
	if err := seedClient.DeleteAllOf(ctx, &corev1.Secret{},
		client.InNamespace(targetNamespace),
		client.MatchingLabels(referencedWorkloadIdentitySecretLabels),
	); err != nil {
		return fmt.Errorf("failed to destroy referenced workload identity secrets in namespace %s: %w", targetNamespace, err)
	}

	return nil
}
