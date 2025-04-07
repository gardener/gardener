// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package workloadidentity

import (
	"context"
	"encoding/json"
	"errors"
	"maps"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	securityv1alpha1constants "github.com/gardener/gardener/pkg/apis/security/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
)

// SecretOption represents a function that is used to configure [Secret] during creation.
type SecretOption func(*Secret) error

// Secret wraps [*corev1.Secret] and represents an object which will be used by
// workloads to request a token for a specific [securityv1alpha1.WorkloadIdentity].
// The created secret is properly annotated and labeled so that the token requestor controller
// for workload identities will pick it up and keep a valid workload identity token stored in it.
type Secret struct {
	secret *corev1.Secret

	labels      map[string]string
	annotations map[string]string

	workloadIdentityName           string
	workloadIdentityNamespace      string
	workloadIdentityProviderType   string
	workloadIdentityContextObject  []byte
	workloadIdentityProviderConfig []byte
}

// NewSecret creates a new workload identity secret that will be recognized
// by the token requestor controller for workload identities which will keep
// a valid workload identity token stored in it.
func NewSecret(name, namespace string, opts ...SecretOption) (*Secret, error) {
	workloadIdentitySecret := &Secret{
		secret: &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Data: make(map[string][]byte),
		},
	}

	var err error
	for _, o := range opts {
		if e := o(workloadIdentitySecret); err != nil {
			err = errors.Join(err, e)
		}
	}

	if len(workloadIdentitySecret.workloadIdentityName) == 0 {
		err = errors.Join(err, errors.New("workload identity name is not set"))
	}

	if len(workloadIdentitySecret.workloadIdentityNamespace) == 0 {
		err = errors.Join(err, errors.New("workload identity namespace is not set"))
	}

	if len(workloadIdentitySecret.workloadIdentityProviderType) == 0 {
		err = errors.Join(err, errors.New("workload identity provider type is not set"))
	}

	if err != nil {
		return nil, err
	}

	return workloadIdentitySecret, nil
}

// WithLabels is an option that can be used to set additional labels to the workload identity secret
// which are not necessarily correlated with workload identity specific logic.
func WithLabels(labels map[string]string) SecretOption {
	return func(s *Secret) error {
		s.labels = maps.Clone(labels)
		return nil
	}
}

// WithAnnotations is an option that can be used to set additional annotations to the workload identity secret
// which are not necessarily correlated with workload identity specific logic.
func WithAnnotations(annotations map[string]string) SecretOption {
	return func(s *Secret) error {
		s.annotations = maps.Clone(annotations)
		return nil
	}
}

// For is an option that correlates the workload identity secret with a specific workload identity.
// This option is required upon creation of such secret.
func For(workloadIdentityName, workloadIdentityNamespace, workloadIdentityProviderType string) SecretOption {
	return func(s *Secret) error {
		s.workloadIdentityName = workloadIdentityName
		s.workloadIdentityNamespace = workloadIdentityNamespace
		s.workloadIdentityProviderType = workloadIdentityProviderType
		return nil
	}
}

// WithProviderConfig is an option that can be used to store
// provider specific information in the workload identity secret.
func WithProviderConfig(providerConfig *runtime.RawExtension) SecretOption {
	return func(s *Secret) error {
		data, err := json.Marshal(providerConfig)
		if err != nil {
			return err
		}
		s.workloadIdentityProviderConfig = data
		return nil
	}
}

// WithContextObject is an option that can be used
// to indicate to the token requestor controller for workload identities
// that requested tokens are going to be used in the context of the passed object.
func WithContextObject(contextObject securityv1alpha1.ContextObject) SecretOption {
	return func(s *Secret) error {
		data, err := json.Marshal(contextObject)
		if err != nil {
			return err
		}
		s.workloadIdentityContextObject = data
		return nil
	}
}

// Reconcile creates or patches the workload identity secret. Based on the struct configuration, it adds
// annotations and labels that are recognized by the token requestor controller for workload identities.
func (s *Secret) Reconcile(ctx context.Context, c client.Client) error {
	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, c, s.secret, func() error {
		s.secret.Type = corev1.SecretTypeOpaque

		// preserve the renew timestamp if present but overwrite all other annotations
		if v, ok := s.secret.Annotations[securityv1alpha1constants.AnnotationWorkloadIdentityTokenRenewTimestamp]; ok {
			s.secret.Annotations = utils.MergeStringMaps(s.annotations, map[string]string{
				securityv1alpha1constants.AnnotationWorkloadIdentityTokenRenewTimestamp: v,
			})
		} else {
			s.secret.Annotations = s.annotations
		}

		metav1.SetMetaDataAnnotation(&s.secret.ObjectMeta, securityv1alpha1constants.AnnotationWorkloadIdentityNamespace, s.workloadIdentityNamespace)
		metav1.SetMetaDataAnnotation(&s.secret.ObjectMeta, securityv1alpha1constants.AnnotationWorkloadIdentityName, s.workloadIdentityName)
		if len(s.workloadIdentityContextObject) > 0 {
			metav1.SetMetaDataAnnotation(&s.secret.ObjectMeta, securityv1alpha1constants.AnnotationWorkloadIdentityContextObject, string(s.workloadIdentityContextObject))
		} else {
			delete(s.secret.Annotations, securityv1alpha1constants.AnnotationWorkloadIdentityContextObject)
		}

		s.secret.Labels = utils.MergeStringMaps(s.labels, map[string]string{
			securityv1alpha1constants.LabelPurpose:                  securityv1alpha1constants.LabelPurposeWorkloadIdentityTokenRequestor,
			securityv1alpha1constants.LabelWorkloadIdentityProvider: s.workloadIdentityProviderType,
		})

		if s.secret.Data == nil {
			s.secret.Data = make(map[string][]byte)
		}

		// preserve the data token key if present but overwrite all other data keys
		if v, ok := s.secret.Data[securityv1alpha1constants.DataKeyToken]; ok {
			s.secret.Data[securityv1alpha1constants.DataKeyToken] = v
		}

		if len(s.workloadIdentityProviderConfig) > 0 {
			s.secret.Data[securityv1alpha1constants.DataKeyConfig] = s.workloadIdentityProviderConfig
		} else {
			delete(s.secret.Data, securityv1alpha1constants.DataKeyConfig)
		}

		return nil
	},
		// The token-requestor might concurrently update the secret token key to populate the token.
		// Hence, we need to use optimistic locking here to ensure we don't accidentally overwrite the concurrent update.
		// ref https://github.com/gardener/gardener/issues/6092#issuecomment-1156244514
		controllerutils.MergeFromOption{MergeFromOption: client.MergeFromWithOptimisticLock{}})
	return err
}
