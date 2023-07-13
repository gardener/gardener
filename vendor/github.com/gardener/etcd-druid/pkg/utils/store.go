// Copyright 2023 SAP SE or an SAP affiliate company
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"context"
	"fmt"
	"strings"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LocalProviderDefaultMountPath is the default path where the buckets directory is mounted.
	LocalProviderDefaultMountPath = "/etc/gardener/local-backupbuckets"
	// EtcdBackupSecretHostPath is the hostPath field in the etcd-backup secret.
	EtcdBackupSecretHostPath = "hostPath"
)

const (
	aws       = "aws"
	azure     = "azure"
	gcp       = "gcp"
	alicloud  = "alicloud"
	openstack = "openstack"
	dell      = "dell"
	openshift = "openshift"
)

const (
	// S3 is a constant for the AWS and S3 compliant storage provider.
	S3 = "S3"
	// ABS is a constant for the Azure storage provider.
	ABS = "ABS"
	// GCS is a constant for the Google storage provider.
	GCS = "GCS"
	// OSS is a constant for the Alicloud storage provider.
	OSS = "OSS"
	// Swift is a constant for the OpenStack storage provider.
	Swift = "Swift"
	// Local is a constant for the Local storage provider.
	Local = "Local"
	// ECS is a constant for the EMC storage provider.
	ECS = "ECS"
	// OCS is a constant for the OpenShift storage provider.
	OCS = "OCS"
)

// GetHostMountPathFromSecretRef returns the hostPath configured for the given store.
func GetHostMountPathFromSecretRef(ctx context.Context, client client.Client, logger logr.Logger, store *druidv1alpha1.StoreSpec, namespace string) (string, error) {
	if store.SecretRef == nil {
		logger.Info("secretRef is not defined for store, using default hostPath", "namespace", namespace)
		return LocalProviderDefaultMountPath, nil
	}

	secret := &corev1.Secret{}
	if err := client.Get(ctx, Key(namespace, store.SecretRef.Name), secret); err != nil {
		return "", err
	}

	hostPath, ok := secret.Data[EtcdBackupSecretHostPath]
	if !ok {
		return LocalProviderDefaultMountPath, nil
	}

	return string(hostPath), nil
}

// StorageProviderFromInfraProvider converts infra to object store provider.
func StorageProviderFromInfraProvider(infra *druidv1alpha1.StorageProvider) (string, error) {
	if infra == nil || len(*infra) == 0 {
		return "", nil
	}

	switch *infra {
	case aws, S3:
		return S3, nil
	case azure, ABS:
		return ABS, nil
	case alicloud, OSS:
		return OSS, nil
	case openstack, Swift:
		return Swift, nil
	case gcp, GCS:
		return GCS, nil
	case dell, ECS:
		return ECS, nil
	case openshift, OCS:
		return OCS, nil
	case Local, druidv1alpha1.StorageProvider(strings.ToLower(Local)):
		return Local, nil
	default:
		return "", fmt.Errorf("unsupported storage provider: %v", *infra)
	}
}
