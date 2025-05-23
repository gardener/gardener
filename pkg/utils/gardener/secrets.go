// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	forkedyaml "github.com/gardener/gardener/third_party/gopkg.in/yaml.v2"
)

var (
	// NoControlPlaneSecretsReq is a label selector requirement to select non-control plane secrets.
	NoControlPlaneSecretsReq = utils.MustNewRequirement(v1beta1constants.GardenRole, selection.NotIn, v1beta1constants.ControlPlaneSecretRoles...)
	// UncontrolledSecretSelector is a selector for objects which are managed by operators/users and not created by
	// Gardener controllers.
	UncontrolledSecretSelector = client.MatchingLabelsSelector{Selector: labels.NewSelector().Add(NoControlPlaneSecretsReq)}
)

// FetchKubeconfigFromSecret tries to retrieve the kubeconfig bytes in given secret.
func FetchKubeconfigFromSecret(ctx context.Context, c client.Client, key client.ObjectKey) ([]byte, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	kubeconfig, ok := secret.Data[kubernetes.KubeConfig]
	if !ok || len(kubeconfig) == 0 {
		return nil, errors.New("the secret's field 'kubeconfig' is either not present or empty")
	}

	return kubeconfig, nil
}

// LabelPurposeGlobalMonitoringSecret is a constant for the value of the purpose label for replicated global monitoring
// secrets.
const LabelPurposeGlobalMonitoringSecret = "global-monitoring-secret-replica"

// ReplicateGlobalMonitoringSecret replicates the global monitoring secret into the given namespace and prefixes it with
// the given prefix.
func ReplicateGlobalMonitoringSecret(ctx context.Context, c client.Client, prefix, namespace string, globalMonitoringSecret *corev1.Secret) (*corev1.Secret, error) {
	globalMonitoringSecretReplica := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: prefix + globalMonitoringSecret.Name, Namespace: namespace}}
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, globalMonitoringSecretReplica, func() error {
		metav1.SetMetaDataLabel(&globalMonitoringSecretReplica.ObjectMeta, v1beta1constants.GardenerPurpose, LabelPurposeGlobalMonitoringSecret)

		globalMonitoringSecretReplica.Type = globalMonitoringSecret.Type
		globalMonitoringSecretReplica.Data = globalMonitoringSecret.Data
		globalMonitoringSecretReplica.Immutable = globalMonitoringSecret.Immutable

		if _, ok := globalMonitoringSecretReplica.Data[secretsutils.DataKeyAuth]; !ok {
			credentials, err := utils.CreateBcryptCredentials(globalMonitoringSecret.Data[secretsutils.DataKeyUserName], globalMonitoringSecret.Data[secretsutils.DataKeyPassword])
			if err != nil {
				return err
			}
			globalMonitoringSecretReplica.Data[secretsutils.DataKeyAuth] = credentials
		}

		return nil
	})
	return globalMonitoringSecretReplica, err
}

var injectionScheme = kubernetes.SeedScheme

// MutateObjectsInSecretData iterates over the given rendered secret data and invokes the given mutate functions.
func MutateObjectsInSecretData(
	secretData map[string][]byte,
	namespace string,
	apiGroups []string,
	mutateFns ...func(object runtime.Object) error,
) error {
	return mutateObjects(secretData, func(obj *unstructured.Unstructured) error {
		// only inject into objects of selected API groups
		if !slices.Contains(apiGroups, obj.GetObjectKind().GroupVersionKind().Group) {
			return nil
		}

		if obj.GetNamespace() != namespace {
			return nil
		}

		return mutateTypedObject(obj, func(typedObject runtime.Object) error {
			for _, mutate := range mutateFns {
				if err := mutate(typedObject); err != nil {
					return err
				}
			}

			return nil
		})
	})
}

// mutateObject iterates over the given rendered secret data and calls the given mutator for each of them. It marshals
// the objects back (with stable key ordering) after mutation and updates the secret data.
func mutateObjects(secretData map[string][]byte, mutate func(obj *unstructured.Unstructured) error) error {
	for key, data := range secretData {
		buffer := &bytes.Buffer{}
		manifestReader := kubernetes.NewManifestReader(data)

		for {
			_, _ = buffer.WriteString("\n---\n")
			obj, err := manifestReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if obj == nil {
				continue
			}

			if err := mutate(obj); err != nil {
				return err
			}

			// serialize unstructured back to secret data (with stable key ordering)
			// Note: we have to do this for all objects, not only for mutated ones, as there could be multiple objects in one file
			objBytes, err := forkedyaml.Marshal(obj.Object)
			if err != nil {
				return err
			}

			if _, err := buffer.Write(objBytes); err != nil {
				return err
			}
		}

		secretData[key] = buffer.Bytes()
	}

	return nil
}

// ObjectsInSecretData reads the given secret data and returns the objects contained in it.
func ObjectsInSecretData(secretData map[string][]byte) ([]runtime.Object, error) {
	var objects []runtime.Object

	for _, data := range secretData {
		buffer := &bytes.Buffer{}
		manifestReader := kubernetes.NewManifestReader(data)

		for {
			_, _ = buffer.WriteString("\n---\n")
			obj, err := manifestReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, err
			}
			if obj == nil {
				continue
			}

			objects = append(objects, obj)
		}
	}

	return objects, nil
}

// mutateTypedObject converts the given object to a typed object, calls the mutator, and converts the object back to the
// original type.
func mutateTypedObject(obj runtime.Object, mutate func(obj runtime.Object) error) error {
	// convert to typed object for injection logic
	typedObject, err := injectionScheme.New(obj.GetObjectKind().GroupVersionKind())
	if err != nil {
		return err
	}
	if err := injectionScheme.Convert(obj, typedObject, nil); err != nil {
		return err
	}

	if err := mutate(typedObject); err != nil {
		return err
	}

	// convert back into unstructured for serialization
	return injectionScheme.Convert(typedObject, obj, nil)
}
