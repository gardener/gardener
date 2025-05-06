// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package migration

import (
	"context"
	"fmt"
	"maps"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// GetPersistedSecrets uses the seedClient to fetch the data of all Secrets that have the `persist` label key set to true
// from the Shoot's control plane namespace
func GetPersistedSecrets(ctx context.Context, seedClient client.Reader, namespace string) (map[string]corev1.Secret, error) {
	secretList := &corev1.SecretList{}
	if err := seedClient.List(
		ctx,
		secretList,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{secretsmanager.LabelKeyPersist: secretsmanager.LabelValueTrue}),
	); err != nil {
		return nil, err
	}

	secretsMap := make(map[string]corev1.Secret, len(secretList.Items))
	for _, secret := range secretList.Items {
		secretsMap[secret.Name] = secret
	}

	return secretsMap, nil
}

// ComparePersistedSecrets ensures that two secret maps are equal.
func ComparePersistedSecrets(secretsBefore, secretsAfter map[string]corev1.Secret) error {
	var errorMsg string
	for name, secret := range secretsBefore {
		if !reflect.DeepEqual(secret.Data, secretsAfter[name].Data) {
			errorMsg += fmt.Sprintf("Secret %s/%s did not have it's data persisted.\n", secret.Namespace, secret.Name)
		}
		if !maps.Equal(secret.Labels, secretsAfter[name].Labels) {
			errorMsg += fmt.Sprintf("Secret %s/%s did not have it's labels persisted: labels before migration: %v, labels after migration: %v\n",
				secret.Namespace,
				secret.Name,
				secret.Labels,
				secretsAfter[name].Labels,
			)
		}
	}
	if len(errorMsg) > 0 {
		return fmt.Errorf("control plane secrets did not have their data or labels persisted during control plane migration:\n %s", errorMsg)
	}
	return nil
}

// CheckForOrphanedNonNamespacedResources checks if there are orphaned resources left on the target seed after the shoot migration.
// The function checks for Cluster, DNSOwner, BackupEntry, ClusterRoleBinding, ClusterRole and PersistentVolume
func CheckForOrphanedNonNamespacedResources(ctx context.Context, shootNamespace string, sourceSeedClient client.Client) error {
	seedClientScheme := sourceSeedClient.Scheme()

	if err := extensionsv1alpha1.AddToScheme(seedClientScheme); err != nil {
		return err
	}

	var leakedObjects []string

	for _, obj := range []client.ObjectList{
		&extensionsv1alpha1.ClusterList{},
		&extensionsv1alpha1.BackupEntryList{},
		&rbacv1.ClusterRoleBindingList{},
		&rbacv1.ClusterRoleList{},
	} {
		if err := sourceSeedClient.List(ctx, obj, client.InNamespace(corev1.NamespaceAll)); err != nil {
			return err
		}

		if err := meta.EachListItem(obj, func(object runtime.Object) error {
			if strings.Contains(object.(client.Object).GetName(), shootNamespace) {
				leakedObjects = append(leakedObjects, fmt.Sprintf("%T %s", object, object.(client.Object).GetName()))
			}
			return nil
		}); err != nil {
			return err
		}
	}

	pvList := &corev1.PersistentVolumeList{}
	if err := sourceSeedClient.List(ctx, pvList, client.InNamespace(corev1.NamespaceAll)); err != nil {
		return err
	}
	if err := meta.EachListItem(pvList, func(obj runtime.Object) error {
		pv := obj.(*corev1.PersistentVolume)
		if strings.Contains(pv.Spec.ClaimRef.Namespace, shootNamespace) {
			leakedObjects = append(leakedObjects, fmt.Sprintf("PersistentVolume/%s", pv.GetName()))
		}
		return nil
	}); err != nil {
		return err
	}
	if len(leakedObjects) > 0 {
		return fmt.Errorf("the following object(s) still exists in the source seed %v", leakedObjects)
	}
	return nil
}
