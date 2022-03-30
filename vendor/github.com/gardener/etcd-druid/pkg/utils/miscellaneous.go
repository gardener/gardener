// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// LocalProviderDefaultMountPath is the default path where the buckets directory is mounted.
	LocalProviderDefaultMountPath = "/etc/gardener/local-backupbuckets"
	// EtcdBackupSecretHostPath is the hostPath field in the etcd-backup secret.
	EtcdBackupSecretHostPath = "hostPath"
)

const (
	crashLoopBackOff = "CrashLoopBackOff"
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
	s3    = "S3"
	abs   = "ABS"
	gcs   = "GCS"
	oss   = "OSS"
	swift = "Swift"
	Local = "Local"
	ecs   = "ECS"
	ocs   = "OCS"
)

// ValueExists returns true or false, depending on whether the given string <value>
// is part of the given []string list <list>.
func ValueExists(value string, list []string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// MergeMaps takes two maps <a>, <b> and merges them. If <b> defines a value with a key
// already existing in the <a> map, the <a> value for that key will be overwritten.
func MergeMaps(a, b map[string]interface{}) map[string]interface{} {
	var values = map[string]interface{}{}

	for i, v := range b {
		existing, ok := a[i]
		values[i] = v

		switch elem := v.(type) {
		case map[string]interface{}:
			if ok {
				if extMap, ok := existing.(map[string]interface{}); ok {
					values[i] = MergeMaps(extMap, elem)
				}
			}
		default:
			values[i] = v
		}
	}

	for i, v := range a {
		if _, ok := values[i]; !ok {
			values[i] = v
		}
	}

	return values
}

// MergeStringMaps merges the content of the newMaps with the oldMap. If a key already exists then
// it gets overwritten by the last value with the same key.
func MergeStringMaps(oldMap map[string]string, newMaps ...map[string]string) map[string]string {
	var out map[string]string

	if oldMap != nil {
		out = make(map[string]string)
	}
	for k, v := range oldMap {
		out[k] = v
	}

	for _, newMap := range newMaps {
		if newMap != nil && out == nil {
			out = make(map[string]string)
		}

		for k, v := range newMap {
			out[k] = v
		}
	}

	return out
}

// TimeElapsed takes a <timestamp> and a <duration> checks whether the elapsed time until now is less than the <duration>.
// If yes, it returns true, otherwise it returns false.
func TimeElapsed(timestamp *metav1.Time, duration time.Duration) bool {
	if timestamp == nil {
		return true
	}

	var (
		end = metav1.NewTime(timestamp.Time.Add(duration))
		now = metav1.Now()
	)
	return !now.Before(&end)
}

func nameAndNamespace(namespaceOrName string, nameOpt ...string) (namespace, name string) {
	if len(nameOpt) > 1 {
		panic(fmt.Sprintf("more than name/namespace for key specified: %s/%v", namespaceOrName, nameOpt))
	}
	if len(nameOpt) == 0 {
		name = namespaceOrName
		return
	}
	namespace = namespaceOrName
	name = nameOpt[0]
	return
}

// Key creates a new client.ObjectKey from the given parameters.
// There are only two ways to call this function:
// - If only namespaceOrName is set, then a client.ObjectKey with name set to namespaceOrName is returned.
// - If namespaceOrName and one nameOpt is given, then a client.ObjectKey with namespace set to namespaceOrName
//   and name set to nameOpt[0] is returned.
// For all other cases, this method panics.
func Key(namespaceOrName string, nameOpt ...string) client.ObjectKey {
	namespace, name := nameAndNamespace(namespaceOrName, nameOpt...)
	return client.ObjectKey{Namespace: namespace, Name: name}
}

// GetStoreValues converts the values in the StoreSpec to a map, or returns an error if the storage provider is unsupported.
func GetStoreValues(ctx context.Context, client client.Client, store *druidv1alpha1.StoreSpec, namespace string) (map[string]interface{}, error) {
	storageProvider, err := StorageProviderFromInfraProvider(store.Provider)
	if err != nil {
		return nil, err
	}
	storeValues := map[string]interface{}{
		"storePrefix":     store.Prefix,
		"storageProvider": storageProvider,
	}
	if strings.EqualFold(string(*store.Provider), Local) {
		mountPath, err := getHostMountPathFromSecretRef(ctx, client, store, namespace)
		if err != nil {
			return nil, err
		}
		storeValues["storageMountPath"] = mountPath
	}
	if store.Container != nil {
		storeValues["storageContainer"] = store.Container
	}
	if store.SecretRef != nil {
		storeValues["storeSecret"] = store.SecretRef.Name
	}
	return storeValues, nil
}

func getHostMountPathFromSecretRef(ctx context.Context, client client.Client, store *druidv1alpha1.StoreSpec, namespace string) (string, error) {
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
	case aws, s3:
		return s3, nil
	case azure, abs:
		return abs, nil
	case alicloud, oss:
		return oss, nil
	case openstack, swift:
		return swift, nil
	case gcp, gcs:
		return gcs, nil
	case dell, ecs:
		return ecs, nil
	case openshift, ocs:
		return ocs, nil
	case Local, druidv1alpha1.StorageProvider(strings.ToLower(Local)):
		return Local, nil
	default:
		return "", fmt.Errorf("unsupported storage provider: %v", *infra)
	}
}

// IsPodInCrashloopBackoff checks if the pod is in CrashloopBackoff from its status fields.
func IsPodInCrashloopBackoff(status v1.PodStatus) bool {
	for _, containerStatus := range status.ContainerStatuses {
		if isContainerInCrashLoopBackOff(containerStatus.State) {
			return true
		}
	}
	return false
}

func isContainerInCrashLoopBackOff(containerState v1.ContainerState) bool {
	if containerState.Waiting != nil {
		return containerState.Waiting.Reason == crashLoopBackOff
	}
	return false
}

// Max returns the larger of x or y.
func Max(x, y int) int {
	if y > x {
		return x
	}
	return x
}
