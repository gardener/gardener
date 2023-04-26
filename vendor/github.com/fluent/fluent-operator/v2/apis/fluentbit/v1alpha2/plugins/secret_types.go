package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-openapi/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// +kubebuilder:object:generate:=true
// Secret defines the key of a value.
type Secret struct {
	ValueFrom ValueSource `json:"valueFrom,omitempty"`
}

// +kubebuilder:object:generate:=true
// ValueSource defines how to find a value's key.
type ValueSource struct {
	// Selects a key of a secret in the pod's namespace
	// +optional
	SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

type SecretLoader struct {
	Client    client.Client
	namespace string
}

func NewSecretLoader(c client.Client, ns string) SecretLoader {
	return SecretLoader{
		Client:    c,
		namespace: ns,
	}
}

func (sl SecretLoader) LoadSecret(s Secret) (string, error) {
	var secret corev1.Secret
	if err := sl.Client.Get(context.Background(), client.ObjectKey{Name: s.ValueFrom.SecretKeyRef.Name, Namespace: sl.namespace}, &secret); err != nil {
		return "", err
	}

	if v, ok := secret.Data[s.ValueFrom.SecretKeyRef.Key]; !ok {
		return "", errors.NotFound(fmt.Sprintf("The key %s is not found.", s.ValueFrom.SecretKeyRef.Key))
	} else {
		return strings.TrimSuffix(fmt.Sprintf("%s", v), "\n"), nil
	}
}
