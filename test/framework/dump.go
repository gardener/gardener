// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package framework

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// DumpDefaultResourcesInAllNamespaces dumps all default k8s resources of a namespace
func (f *CommonFramework) DumpDefaultResourcesInAllNamespaces(ctx context.Context, k8sClient kubernetes.Interface) error {
	namespaces := &corev1.NamespaceList{}
	if err := k8sClient.Client().List(ctx, namespaces); err != nil {
		return err
	}

	var result error

	for _, ns := range namespaces.Items {
		if err := f.DumpDefaultResourcesInNamespace(ctx, k8sClient, ns.Name); err != nil {
			result = multierror.Append(result, err)
		}
	}
	return result
}

// DumpDefaultResourcesInNamespace dumps all default K8s resources of a namespace.
func (f *CommonFramework) DumpDefaultResourcesInNamespace(ctx context.Context, k8sClient kubernetes.Interface, namespace string) error {
	log := f.Logger.WithValues("namespace", namespace)

	var result error
	if err := f.dumpEventsInNamespace(ctx, log, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch Events from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpPodInfoForNamespace(ctx, log, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Pods from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpDeploymentInfoForNamespace(ctx, log, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Deployments from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpStatefulSetInfoForNamespace(ctx, log, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of StatefulSets from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpDaemonSetInfoForNamespace(ctx, log, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of DaemonSets from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpServiceInfoForNamespace(ctx, log, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Services from namespace %s: %s", namespace, err.Error()))
	}
	if err := f.dumpVolumeInfoForNamespace(ctx, log, k8sClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Volumes from namespace %s: %s", namespace, err.Error()))
	}
	return result
}

func (f *GardenerFramework) dumpControlplaneInSeed(ctx context.Context, seed *gardencorev1beta1.Seed, namespace string) error {
	log := f.Logger.WithValues("seedName", seed.GetName(), "namespace", namespace)
	log.Info("Dumping control plane resources")

	_, seedClient, err := f.GetSeed(ctx, seed.GetName())
	if err != nil {
		return err
	}

	var result error
	if err := f.dumpGardenerExtensionsInNamespace(ctx, log, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump Extensions from namespace %s in seed %s: %w", namespace, seed.Name, err))
	}
	if err := f.dumpEventsInNamespace(ctx, log, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump Events from namespace %s in seed %s: %w", namespace, seed.Name, err))
	}
	if err := f.dumpPodInfoForNamespace(ctx, log, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of Pods from namespace %s in seed %s: %w", namespace, seed.Name, err))
	}
	if err := f.dumpDeploymentInfoForNamespace(ctx, log, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of Deployments from namespace %s in seed %s: %w", namespace, seed.Name, err))
	}
	if err := f.dumpStatefulSetInfoForNamespace(ctx, log, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of StatefulSets from namespace %s in seed %s: %w", namespace, seed.Name, err))
	}
	if err := f.dumpDaemonSetInfoForNamespace(ctx, log, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of DaemonSets from namespace %s in seed %s: %w", namespace, seed.Name, err))
	}
	if err := f.dumpServiceInfoForNamespace(ctx, log, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to dump information of Services from namespace %s in seed %s: %w", namespace, seed.Name, err))
	}
	if err := f.dumpVolumeInfoForNamespace(ctx, log, seedClient, namespace); err != nil {
		result = multierror.Append(result, fmt.Errorf("unable to fetch information of Volumes from namespace %s: %w", namespace, err))
	}

	return result
}

// dumpGardenerExtensionsInNamespace prints all gardener extension crds in the shoot namespace
func (f *GardenerFramework) dumpGardenerExtensionsInNamespace(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string) error {
	var result *multierror.Error

	for kind, objList := range map[string]client.ObjectList{
		"Infrastructure":        &extensionsv1alpha1.InfrastructureList{},
		"ControlPlane":          &extensionsv1alpha1.ControlPlaneList{},
		"OperatingSystemConfig": &extensionsv1alpha1.OperatingSystemConfigList{},
		"Worker":                &extensionsv1alpha1.WorkerList{},
		"BackupBucket":          &extensionsv1alpha1.BackupBucketList{},
		"BackupEntry":           &extensionsv1alpha1.BackupEntryList{},
		"Bastion":               &extensionsv1alpha1.BastionList{},
		"Network":               &extensionsv1alpha1.NetworkList{},
	} {
		extensionLog := log.WithValues("kind", kind)
		extensionLog.Info("Dumping extensions.gardener.cloud/v1alpha1 resources")

		if err := k8sClient.Client().List(ctx, objList, client.InNamespace(namespace)); err != nil {
			result = multierror.Append(result, err)
			if err := meta.EachListItem(objList, func(o runtime.Object) error {
				obj, err := extensions.Accessor(o)
				if err != nil {
					return err
				}
				f.dumpGardenerExtension(extensionLog, obj)
				return nil
			}); err != nil {
				result = multierror.Append(result, err)
			}
		}
	}

	return result.ErrorOrNil()
}

// dumpGardenerExtensions prints all gardener extension crds in the shoot namespace
func (f *GardenerFramework) dumpGardenerExtension(log logr.Logger, extension extensionsv1alpha1.Object) {
	log = log.WithValues("name", extension.GetName(), "type", extension.GetExtensionSpec().GetExtensionType())

	if err := health.CheckExtensionObject(extension); err != nil {
		log.Info("Found unhealthy extension object", "reason", err.Error())
	} else {
		log.Info("Found healthy extension object")
	}

	log.Info("Extension object has last operation", "lastOperation", extension.GetExtensionStatus().GetLastOperation())
	if extension.GetExtensionStatus().GetLastError() != nil {
		log.Info("Extension object has last error", "lastError", extension.GetExtensionStatus().GetLastError())
	}
}

// DumpLogsForPodsWithLabelsInNamespace prints the logs of all containers of pods in the given namespace selected by the given list options.
func (f *CommonFramework) DumpLogsForPodsWithLabelsInNamespace(ctx context.Context, k8sClient kubernetes.Interface, namespace string, opts ...client.ListOption) error {
	pods := &corev1.PodList{}
	opts = append(opts, client.InNamespace(namespace))
	if err := k8sClient.Client().List(ctx, pods, opts...); err != nil {
		return err
	}

	var result error
	for _, pod := range pods.Items {
		for _, container := range pod.Spec.InitContainers {
			if err := f.DumpLogsForPodInNamespace(ctx, k8sClient, namespace, pod.Name, &corev1.PodLogOptions{Container: container.Name}); err != nil {
				result = multierror.Append(result, fmt.Errorf("error reading logs from pod %q init container %q: %w", pod.Name, container.Name, err))
			}
		}
		for _, container := range pod.Spec.Containers {
			if err := f.DumpLogsForPodInNamespace(ctx, k8sClient, namespace, pod.Name, &corev1.PodLogOptions{Container: container.Name}); err != nil {
				result = multierror.Append(result, fmt.Errorf("error reading logs from pod %q container %q: %w", pod.Name, container.Name, err))
			}
		}
	}
	return result
}

// DumpLogsForPodInNamespace prints the logs of the pod with the given namespace and name.
func (f *CommonFramework) DumpLogsForPodInNamespace(ctx context.Context, k8sClient kubernetes.Interface, namespace, name string, options *corev1.PodLogOptions) error {
	log := f.Logger.WithValues("pod", client.ObjectKey{Namespace: namespace, Name: name})
	if options != nil && options.Container != "" {
		log = log.WithValues("container", options.Container)
	}
	log.Info("Dumping logs for corev1.Pod")

	podIf := k8sClient.Kubernetes().CoreV1().Pods(namespace)
	logs, err := kubernetesutils.GetPodLogs(ctx, podIf, name, options)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(bytes.NewReader(logs))
	for scanner.Scan() {
		log.Info(scanner.Text()) //nolint:logcheck
	}

	return nil
}

// dumpDeploymentInfoForNamespace prints information about all Deployments of a namespace
func (f *CommonFramework) dumpDeploymentInfoForNamespace(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string) error {
	log.Info("Dumping appsv1.Deployment resources")

	deployments := &appsv1.DeploymentList{}
	if err := k8sClient.Client().List(ctx, deployments, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, deployment := range deployments.Items {
		objectLog := log.WithValues("name", deployment.Name, "replicas", deployment.Status.Replicas, "availableReplicas", deployment.Status.AvailableReplicas)

		if err := health.CheckDeployment(&deployment); err != nil {
			objectLog.Info("Found unhealthy Deployment", "reason", err.Error(), "conditions", deployment.Status.Conditions)
			continue
		}
		objectLog.Info("Found healthy Deployment")
	}
	return nil
}

// dumpStatefulSetInfoForNamespace prints information about all StatefulSets of a namespace
func (f *CommonFramework) dumpStatefulSetInfoForNamespace(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string) error {
	log.Info("Dumping appsv1.StatefulSet resources")

	statefulSets := &appsv1.StatefulSetList{}
	if err := k8sClient.Client().List(ctx, statefulSets, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, statefulSet := range statefulSets.Items {
		objectLog := log.WithValues("name", statefulSet.Name, "replicas", statefulSet.Status.Replicas, "readyReplicas", statefulSet.Status.ReadyReplicas)

		if err := health.CheckStatefulSet(&statefulSet); err != nil {
			objectLog.Info("Found unhealthy StatefulSet", "reason", err.Error(), "conditions", statefulSet.Status.Conditions)
			continue
		}
		objectLog.Info("Found healthy StatefulSet")
	}
	return nil
}

// dumpDaemonSetInfoForNamespace prints information about all DaemonSets of a namespace
func (f *CommonFramework) dumpDaemonSetInfoForNamespace(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string) error {
	log.Info("Dumping appsv1.DaemonSet resources")

	daemonSets := &appsv1.DaemonSetList{}
	if err := k8sClient.Client().List(ctx, daemonSets, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, ds := range daemonSets.Items {
		objectLog := log.WithValues("name", ds.Name, "currentNumberScheduled", ds.Status.CurrentNumberScheduled, "desiredNumberScheduled", ds.Status.DesiredNumberScheduled)

		if err := health.CheckDaemonSet(&ds); err != nil {
			objectLog.Info("Found unhealthy DaemonSet", "reason", err.Error(), "conditions", ds.Status.Conditions)
			continue
		}
		objectLog.Info("Found healthy DaemonSet")
	}
	return nil
}

// dumpNamespaceResource prints information about the Namespace itself
func (f *CommonFramework) dumpNamespaceResource(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string) error {
	log.Info("Dumping corev1.Namespace resources")

	ns := &corev1.Namespace{}
	if err := k8sClient.Client().Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		return err
	}
	log.Info("Found Namespace", "namespace", ns)
	return nil
}

// dumpServiceInfoForNamespace prints information about all Services of a namespace
func (f *CommonFramework) dumpServiceInfoForNamespace(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string) error {
	log.Info("Dumping corev1.Service resources")

	services := &corev1.ServiceList{}
	if err := k8sClient.Client().List(ctx, services, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, service := range services.Items {
		log.Info("Found Service", "service", service)
	}
	return nil
}

// dumpVolumeInfoForNamespace prints information about all PVs and PVCs of a namespace
func (f *CommonFramework) dumpVolumeInfoForNamespace(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string) error {
	log.Info("Dumping corev1.PersistentVolumeClaim resources")

	pvcs := &corev1.PersistentVolumeClaimList{}
	if err := k8sClient.Client().List(ctx, pvcs, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, pvc := range pvcs.Items {
		log.Info("Found PersistentVolumeClaim", "persistentVolumeClaim", pvc)
	}

	log.Info("Dumping corev1.PersistentVolume resources")

	pvs := &corev1.PersistentVolumeList{}
	if err := k8sClient.Client().List(ctx, pvs, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, pv := range pvs.Items {
		log.Info("Found PersistentVolume", "persistentVolume", pv)
	}
	return nil
}

// dumpNodes prints information about all nodes
func (f *CommonFramework) dumpNodes(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface) error {
	log.Info("Dumping corev1.Node resources")

	nodes := &corev1.NodeList{}
	if err := k8sClient.Client().List(ctx, nodes); err != nil {
		return err
	}
	for _, node := range nodes.Items {
		objectLog := log.WithValues("nodeName", node.Name)
		if err := health.CheckNode(&node); err != nil {
			objectLog.Info("Found unhealthy Node", "phase", node.Status.Phase, "reason", err.Error(), "conditions", node.Status.Conditions)
		} else {
			objectLog.Info("Found healthy Node", "phase", node.Status.Phase)
		}
		objectLog.Info("Node resource capacity", "cpu", node.Status.Capacity.Cpu().String(), "memory", node.Status.Capacity.Memory().String())

		nodeMetric := &metricsv1beta1.NodeMetrics{}
		if err := k8sClient.Client().Get(ctx, client.ObjectKey{Name: node.Name}, nodeMetric); err != nil {
			objectLog.Error(err, "Unable to receive metrics for node")
			continue
		}
		objectLog.Info("Node resource usage", "cpu", nodeMetric.Usage.Cpu().String(), "memory", nodeMetric.Usage.Memory().String())
	}
	return nil
}

// dumpPodInfoForNamespace prints node information of all pods in a namespace
func (f *CommonFramework) dumpPodInfoForNamespace(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string) error {
	log.Info("Dumping corev1.Pod resources")

	pods := &corev1.PodList{}
	if err := k8sClient.Client().List(ctx, pods, client.InNamespace(namespace)); err != nil {
		return err
	}
	for _, pod := range pods.Items {
		log.Info("Found pod",
			"podName", pod.Name,
			"phase", pod.Status.Phase,
			"nodeName", pod.Spec.NodeName,
		)
	}
	return nil
}

// dumpEventsInNamespace prints all events of a namespace
func (f *CommonFramework) dumpEventsInNamespace(ctx context.Context, log logr.Logger, k8sClient kubernetes.Interface, namespace string, filters ...EventFilterFunc) error {
	log.Info("Dumping corev1.Event resources")

	events := &corev1.EventList{}
	if err := k8sClient.Client().List(ctx, events, client.InNamespace(namespace)); err != nil {
		return err
	}

	if len(events.Items) > 1 {
		sort.Sort(eventByFirstTimestamp(events.Items))
	}
	for _, event := range events.Items {
		if ApplyFilters(event, filters...) {
			log.Info("Found event",
				"firstTimestamp", event.FirstTimestamp,
				"involvedObjectName", event.InvolvedObject.Name,
				"source", event.Source,
				"reason", event.Reason,
				"message", event.Message,
			)
		}
	}
	return nil
}

// EventFilterFunc is a function to filter events
type EventFilterFunc func(event corev1.Event) bool

// ApplyFilters checks if one of the EventFilters filters the current event
func ApplyFilters(event corev1.Event, filters ...EventFilterFunc) bool {
	for _, filter := range filters {
		if !filter(event) {
			return false
		}
	}
	return true
}

// eventByFirstTimestamp sorts a slice of events by first timestamp, using their involvedObject's name as a tie breaker.
type eventByFirstTimestamp []corev1.Event

func (o eventByFirstTimestamp) Len() int { return len(o) }

func (o eventByFirstTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }

func (o eventByFirstTimestamp) Less(i, j int) bool {
	if o[i].FirstTimestamp.Equal(&o[j].FirstTimestamp) {
		return o[i].InvolvedObject.Name < o[j].InvolvedObject.Name
	}
	return o[i].FirstTimestamp.Before(&o[j].FirstTimestamp)
}
