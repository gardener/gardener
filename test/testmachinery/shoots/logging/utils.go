// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/gardener/gardener/test/framework"
)

func getFluentBitDaemonSet(ctx context.Context, k8sSeedClient kubernetes.Interface) (*appsv1.DaemonSet, error) {
	daemonSetList := &appsv1.DaemonSetList{}
	err := k8sSeedClient.Client().List(ctx,
		daemonSetList,
		client.InNamespace(garden),
		client.MatchingLabels{
			v1beta1constants.LabelApp:   v1beta1constants.DaemonSetNameFluentBit,
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleLogging,
		})
	if err != nil {
		return nil, err
	}
	if len(daemonSetList.Items) == 0 {
		return nil, fmt.Errorf("fluent-bit daemonset not found")
	}
	return daemonSetList.Items[0].DeepCopy(), nil
}

func isFluentBitPresent(ctx context.Context, k8sSeedClient kubernetes.Interface) (bool, error) {
	_, err := getFluentBitDaemonSet(ctx, k8sSeedClient)
	if err != nil {
		return false, err
	}
	return true, nil
}

func isValiPresent(ctx context.Context, seedClient client.Client) (bool, error) {
	vali := &appsv1.StatefulSet{}
	if err := seedClient.Get(ctx, client.ObjectKey{Namespace: garden, Name: valiName}, vali); err != nil {
		return false, err
	}
	return true, nil
}

func isOtelCollectorPresent(ctx context.Context, seedClient client.Client) (bool, error) {
	otelDeploy := &appsv1.Deployment{}
	if err := seedClient.Get(ctx, client.ObjectKey{Namespace: garden, Name: otelCollectorDeploymentName}, otelDeploy); err != nil {
		return false, err
	}
	return true, nil
}

// WaitUntilValiReceivesLogs waits until the vali instance in <valiNamespace> receives <expected> logs for <key>=<value>
func WaitUntilValiReceivesLogs(ctx context.Context, interval time.Duration, shootFramework *framework.ShootFramework, valiLabels map[string]string, valiNamespace, key, value string, expected, delta int, c kubernetes.Interface) error {
	err := retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		search, err := shootFramework.GetValiLogs(ctx, valiLabels, valiNamespace, key, value, c)
		if err != nil {
			return retry.SevereError(err)
		}
		var actual int
		for _, result := range search.Data.Result {
			currentStr, ok := result.Value[1].(string)
			if !ok {
				return retry.SevereError(fmt.Errorf("Data.Result.Value[1] is not a string for %s=%s", key, value))
			}
			current, err := strconv.Atoi(currentStr)
			if err != nil {
				return retry.SevereError(fmt.Errorf("Data.Result.Value[1] string is not parsable to integer for %s=%s", key, value))
			}
			actual += current
		}

		log := shootFramework.Logger.WithValues("expected", expected, "actual", actual)

		if expected > actual {
			log.Info("Waiting to receive all expected logs")
			return retry.MinorError(fmt.Errorf("received only %d/%d logs", actual, expected))
		} else if expected+delta < actual {
			return retry.SevereError(fmt.Errorf("expected to receive %d logs but was %d", expected, actual))
		}

		log.Info("Received logs", "delta", delta)
		return retry.Ok()
	})

	if err != nil {
		// ctx might have been cancelled already, make sure we still dump logs, so use context.Background()
		dumpLogsCtx, dumpLogsCancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer dumpLogsCancel()

		shootFramework.Logger.Info("Dump Vali logs")
		if dumpError := shootFramework.DumpLogsForPodInNamespace(dumpLogsCtx, c, valiNamespace, "vali-0",
			&corev1.PodLogOptions{Container: "vali"}); dumpError != nil {
			shootFramework.Logger.Error(dumpError, "Error dumping logs for pod")
		}

		shootFramework.Logger.Info("Dump Fluent-bit logs")
		labels := client.MatchingLabels{"app": "fluent-bit"}
		if dumpError := shootFramework.DumpLogsForPodsWithLabelsInNamespace(dumpLogsCtx, c, "garden",
			labels); dumpError != nil {
			shootFramework.Logger.Error(dumpError, "Error dumping logs for pod")
		}
	}

	return err
}

func encode(obj runtime.Object) []byte {
	data, _ := json.Marshal(obj)
	return data
}

func create(ctx context.Context, c client.Client, obj client.Object) error {
	obj.SetResourceVersion("")
	return client.IgnoreAlreadyExists(c.Create(ctx, obj))
}

func getShootNamespace(number int) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s%v", simulatedShootNamespacePrefix, number),
		},
	}
}

func getCluster(number int) *extensionsv1alpha1.Cluster {
	shoot := &gardencorev1beta1.Shoot{
		Spec: gardencorev1beta1.ShootSpec{
			Hibernation: &gardencorev1beta1.Hibernation{
				Enabled: new(false),
			},
			Purpose: new(gardencorev1beta1.ShootPurpose("evaluation")),
		},
		Status: gardencorev1beta1.ShootStatus{
			LastOperation: &gardencorev1beta1.LastOperation{
				Progress: 50,
				// The logs are sent to central vali only if the operation is Create.
				Type: gardencorev1beta1.LastOperationTypeCreate,
			},
		},
	}

	return &extensionsv1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "extensions.gardener.cloud/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s%v", simulatedShootNamespacePrefix, number),
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			Shoot: runtime.RawExtension{
				Raw: encode(shoot),
			},
			CloudProfile: runtime.RawExtension{
				Raw: encode(&gardencorev1beta1.CloudProfile{}),
			},
			Seed: &runtime.RawExtension{
				Raw: encode(&gardencorev1beta1.Seed{}),
			},
		},
	}
}

func getLoggingShootService(number int) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "logging",
			Namespace: fmt.Sprintf("%s%v", simulatedShootNamespacePrefix, number),
		},
		Spec: corev1.ServiceSpec{
			Type:         corev1.ServiceTypeExternalName,
			ExternalName: "logging-shoot.garden.svc.cluster.local",
		},
	}
}

func getLogCountFromResult(search *framework.SearchResponse) (int, error) {
	var totalLogs int
	for _, result := range search.Data.Result {
		currentStr, ok := result.Value[1].(string)
		if !ok {
			return totalLogs, fmt.Errorf("Data.Result.Value[1] is not a string")
		}
		current, err := strconv.Atoi(currentStr)
		if err != nil {
			return totalLogs, fmt.Errorf("Data.Result.Value[1] string is not parsable to integer")
		}
		totalLogs += current
	}
	return totalLogs, nil
}

func getConfigMapName(volumes []corev1.Volume, wantedVolumeName string) string {
	for _, volume := range volumes {
		if volume.Name == wantedVolumeName && volume.ConfigMap != nil {
			return volume.ConfigMap.Name
		}
	}
	return ""
}

func getSecretNameFromVolume(volumes []corev1.Volume, wantedVolumeName string) string {
	for _, volume := range volumes {
		if volume.Name == wantedVolumeName && volume.Secret != nil {
			return volume.Secret.SecretName
		}
	}
	return ""
}

func newEmptyDirVolume(name, size string) corev1.Volume {
	return corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				SizeLimit: new(resource.MustParse(size)),
			},
		},
	}
}

func newPodAntiAffinity(matchLabels map[string]string) *corev1.PodAntiAffinity {
	return &corev1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
			{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: matchLabels,
				},
				TopologyKey: "kubernetes.io/hostname",
			},
		},
	}
}

func newGardenNamespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

const valiYaml = `
auth_enabled: false
ingester:
  chunk_target_size: 1536000
  chunk_idle_period: 3m
  chunk_block_size: 262144
  chunk_retain_period: 3m
  max_transfer_retries: 3
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
    final_sleep: 0s
    min_ready_duration: 1s
limits_config:
  enforce_metric_name: false
  reject_old_samples: true
  reject_old_samples_max_age: 168h
schema_config:
  configs:
  - from: 2018-04-15
    store: boltdb
    object_store: filesystem
    schema: v11
    index:
      prefix: index_
      period: 24h
server:
  http_listen_port: 3100
storage_config:
  boltdb:
    directory: /data/vali/index
  filesystem:
    directory: /data/vali/chunks
chunk_store_config:
  max_look_back_period: 360h
table_manager:
  retention_deletes_enabled: true
  retention_period: 360h
`
