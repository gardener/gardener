// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
)

// filesystemSecretsManagerClient is a client.Client implementation that persists
// corev1.Secret objects to the filesystem instead of a Kubernetes API server.
// It is used to back the secrets manager when no API server is reachable (self-hosted shoot nodes).
//
// Only the operations called by the secrets manager (Get, Create, List, Patch, Delete) are
// implemented. All other methods are not implemented and will panic if called.
type filesystemSecretsManagerClient struct {
	client.Client

	fs afero.Afero
}

const secretsDir = "/var/lib/etcd" // #nosec G101 -- No credential.

func secretFilePath(namespace, name string) string {
	return filepath.Join(secretsDir, fmt.Sprintf("secret-%s-%s.yaml", namespace, name))
}

// NewFilesystemSecretsManagerClient returns a client.Client that persists Secret objects to the filesystem at
// /var/lib/etcd instead of a Kubernetes API server.
func NewFilesystemSecretsManagerClient(fs afero.Afero) client.Client {
	return &filesystemSecretsManagerClient{fs: fs}
}

func (c *filesystemSecretsManagerClient) Get(_ context.Context, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("filesystemSecretsManagerClient.Get: unsupported object type %T", obj)
	}

	data, err := c.fs.ReadFile(secretFilePath(key.Namespace, key.Name))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return apierrors.NewNotFound(corev1.Resource("secrets"), key.Name)
		}
		return fmt.Errorf("failed reading secret file for %s/%s: %w", key.Namespace, key.Name, err)
	}

	if _, _, err := kubernetes.ShootCodec.UniversalDeserializer().Decode(data, nil, secret); err != nil {
		return fmt.Errorf("failed decoding secret %s/%s: %w", key.Namespace, key.Name, err)
	}
	return nil
}

func (c *filesystemSecretsManagerClient) Create(_ context.Context, obj client.Object, _ ...client.CreateOption) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("filesystemSecretsManagerClient.Create: unsupported object type %T", obj)
	}

	path := secretFilePath(secret.Namespace, secret.Name)
	if exists, err := c.fs.Exists(path); err != nil {
		return fmt.Errorf("failed checking existence of secret file %s: %w", path, err)
	} else if exists {
		return apierrors.NewAlreadyExists(corev1.Resource("secrets"), secret.Name)
	}

	return c.writeSecret(secret)
}

func (c *filesystemSecretsManagerClient) List(_ context.Context, list client.ObjectList, opts ...client.ListOption) error {
	secretList, ok := list.(*corev1.SecretList)
	if !ok {
		return fmt.Errorf("filesystemSecretsManagerClient.List: unsupported list type %T", list)
	}

	listOpts := &client.ListOptions{}
	for _, o := range opts {
		o.ApplyToList(listOpts)
	}

	entries, err := c.fs.ReadDir(secretsDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed reading secrets directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") || !strings.HasPrefix(entry.Name(), "secret-") {
			continue
		}

		data, err := c.fs.ReadFile(filepath.Join(secretsDir, entry.Name()))
		if err != nil {
			return fmt.Errorf("failed reading secret file %s: %w", entry.Name(), err)
		}

		secret := &corev1.Secret{}
		if _, _, err := kubernetes.ShootCodec.UniversalDeserializer().Decode(data, nil, secret); err != nil {
			return fmt.Errorf("failed decoding secret from %s: %w", entry.Name(), err)
		}

		if listOpts.Namespace != "" && secret.Namespace != listOpts.Namespace {
			continue
		}

		if listOpts.LabelSelector != nil && !listOpts.LabelSelector.Matches(labels.Set(secret.Labels)) {
			continue
		}

		secretList.Items = append(secretList.Items, *secret)
	}

	return nil
}

func (c *filesystemSecretsManagerClient) Patch(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("filesystemSecretsManagerClient.Patch: unsupported object type %T", obj)
	}

	if patch.Type() != types.MergePatchType {
		return fmt.Errorf("filesystemSecretsManagerClient.Patch: unsupported patch type %s", patch.Type())
	}

	patchData, err := patch.Data(obj)
	if err != nil {
		return fmt.Errorf("failed computing patch data: %w", err)
	}

	path := secretFilePath(secret.Namespace, secret.Name)
	existingData, err := c.fs.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return apierrors.NewNotFound(corev1.Resource("secrets"), secret.Name)
		}
		return fmt.Errorf("failed reading secret file %s for patch: %w", path, err)
	}

	merged, err := applyMergePatch(existingData, patchData)
	if err != nil {
		return fmt.Errorf("failed applying merge patch to secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	if _, _, err := kubernetes.ShootCodec.UniversalDeserializer().Decode(merged, nil, secret); err != nil {
		return fmt.Errorf("failed decoding merged secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	return c.writeSecret(secret)
}

func (c *filesystemSecretsManagerClient) Delete(_ context.Context, obj client.Object, _ ...client.DeleteOption) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("filesystemSecretsManagerClient.Delete: unsupported object type %T", obj)
	}

	path := secretFilePath(secret.Namespace, secret.Name)
	if err := c.fs.Remove(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return apierrors.NewNotFound(corev1.Resource("secrets"), secret.Name)
		}
		return fmt.Errorf("failed deleting secret file %s: %w", path, err)
	}
	return nil
}

func (c *filesystemSecretsManagerClient) writeSecret(secret *corev1.Secret) error {
	encoder := kubernetes.ShootCodec.LegacyCodec(corev1.SchemeGroupVersion)

	var buf bytes.Buffer
	if err := encoder.Encode(secret, &buf); err != nil {
		return fmt.Errorf("failed encoding secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	path := secretFilePath(secret.Namespace, secret.Name)
	tmpPath := path + ".tmp"

	if err := c.fs.MkdirAll(secretsDir, 0750); err != nil {
		return fmt.Errorf("failed creating secrets directory %s: %w", secretsDir, err)
	}

	if err := c.fs.WriteFile(tmpPath, buf.Bytes(), 0640); err != nil {
		return fmt.Errorf("failed writing secret file %s: %w", tmpPath, err)
	}

	if err := c.fs.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed renaming secret file %s to %s: %w", tmpPath, path, err)
	}

	return nil
}

func applyMergePatch(base, patch []byte) ([]byte, error) {
	var baseMap, patchMap map[string]any
	if err := json.Unmarshal(base, &baseMap); err != nil {
		return nil, fmt.Errorf("failed unmarshalling base: %w", err)
	}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, fmt.Errorf("failed unmarshalling patch: %w", err)
	}
	mergeMaps(baseMap, patchMap)
	return json.Marshal(baseMap)
}

// mergeMaps applies src on top of dst following RFC 7396 merge patch semantics:
// nil values in src delete the key from dst; non-nil values replace or recurse.
func mergeMaps(dst, src map[string]any) {
	for k, v := range src {
		if v == nil {
			delete(dst, k)
			continue
		}
		if srcSub, ok := v.(map[string]any); ok {
			if dstSub, ok := dst[k].(map[string]any); ok {
				mergeMaps(dstSub, srcSub)
				continue
			}
		}
		dst[k] = v
	}
}
