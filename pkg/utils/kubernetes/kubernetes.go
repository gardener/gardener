// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/intstr"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TruncateLabelValue truncates a string at 63 characters so it's suitable for a label value.
func TruncateLabelValue(s string) string {
	if len(s) > 63 {
		return s[:63]
	}
	return s
}

// SetMetaDataLabel sets the key value pair in the labels section of the given Object.
// If the given Object did not yet have labels, they are initialized.
func SetMetaDataLabel(meta metav1.Object, key, value string) {
	labels := meta.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = value
	meta.SetLabels(labels)
}

// SetMetaDataAnnotation sets the annotation on the given object.
// If the given Object did not yet have annotations, they are initialized.
func SetMetaDataAnnotation(meta metav1.Object, key, value string) {
	annotations := meta.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[key] = value
	meta.SetAnnotations(annotations)
}

// HasMetaDataAnnotation checks if the passed meta object has the given key, value set in the annotations section.
func HasMetaDataAnnotation(meta metav1.Object, key, value string) bool {
	val, ok := meta.GetAnnotations()[key]
	return ok && val == value
}

// HasDeletionTimestamp checks if an object has a deletion timestamp
func HasDeletionTimestamp(obj runtime.Object) (bool, error) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return false, err
	}
	return metadata.GetDeletionTimestamp() != nil, nil
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

// ObjectMeta creates a new metav1.ObjectMeta from the given parameters.
// There are only two ways to call this function:
// - If only namespaceOrName is set, then a metav1.ObjectMeta with name set to namespaceOrName is returned.
// - If namespaceOrName and one nameOpt is given, then a metav1.ObjectMeta with namespace set to namespaceOrName
//   and name set to nameOpt[0] is returned.
// For all other cases, this method panics.
func ObjectMeta(namespaceOrName string, nameOpt ...string) metav1.ObjectMeta {
	namespace, name := nameAndNamespace(namespaceOrName, nameOpt...)
	return metav1.ObjectMeta{Namespace: namespace, Name: name}
}

// ObjectMetaFromKey returns an ObjectMeta with the namespace and name set to the values from the key.
func ObjectMetaFromKey(key client.ObjectKey) metav1.ObjectMeta {
	return ObjectMeta(key.Namespace, key.Name)
}

// WaitUntilResourceDeleted deletes the given resource and then waits until it has been deleted. It respects the
// given interval and timeout.
func WaitUntilResourceDeleted(ctx context.Context, c client.Client, obj client.Object, interval time.Duration) error {
	key := client.ObjectKeyFromObject(obj)
	return retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		if err := c.Get(ctx, key, obj); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}
		return retry.MinorError(fmt.Errorf("resource %s still exists", key.String()))
	})
}

// WaitUntilResourcesDeleted waits until the given resources are gone.
// It respects the given interval and timeout.
func WaitUntilResourcesDeleted(ctx context.Context, c client.Client, list client.ObjectList, interval time.Duration, opts ...client.ListOption) error {
	return retry.Until(ctx, interval, func(ctx context.Context) (done bool, err error) {
		if err := c.List(ctx, list, opts...); err != nil {
			return retry.SevereError(err)
		}
		if meta.LenList(list) == 0 {
			return retry.Ok()
		}
		var remainingItems []string
		acc := meta.NewAccessor()
		if err := meta.EachListItem(list, func(remainingObj runtime.Object) error {
			name, err := acc.Name(remainingObj)
			if err != nil {
				return err
			}
			remainingItems = append(remainingItems, name)
			return nil
		}); err != nil {
			return retry.SevereError(err)
		}
		return retry.MinorError(fmt.Errorf("resource(s) %s still exists", remainingItems))
	})
}

// WaitUntilResourceDeletedWithDefaults deletes the given resource and then waits until it has been deleted. It
// uses a default interval and timeout
func WaitUntilResourceDeletedWithDefaults(ctx context.Context, c client.Client, obj client.Object) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	return WaitUntilResourceDeleted(ctx, c, obj, 5*time.Second)
}

// WaitUntilLoadBalancerIsReady waits until the given external load balancer has
// been created (i.e., its ingress information has been updated in the service status).
func WaitUntilLoadBalancerIsReady(ctx context.Context, kubeClient kubernetes.Interface, namespace, name string, timeout time.Duration, logger *logrus.Entry) (string, error) {
	var (
		loadBalancerIngress string
		service             = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	)
	if err := retry.UntilTimeout(ctx, 5*time.Second, timeout, func(ctx context.Context) (done bool, err error) {
		loadBalancerIngress, err = GetLoadBalancerIngress(ctx, kubeClient.Client(), service)
		if err != nil {
			logger.Infof("Waiting until the %s service deployed is ready...", name)
			// TODO(AC): This is a quite optimistic check / we should differentiate here
			return retry.MinorError(fmt.Errorf("%s service deployed is not ready: %v", name, err))
		}
		return retry.Ok()
	}); err != nil {
		const eventsLimit = 2

		eventsErrorMessage, err2 := FetchEventMessages(ctx, kubeClient.DirectClient(), service, corev1.EventTypeWarning, eventsLimit)
		if err2 != nil {
			return "", fmt.Errorf("error '%v' occured while fetching more details on error '%v'", err2, err)
		}
		if eventsErrorMessage != "" {
			errorMessage := err.Error() + "\n\n" + eventsErrorMessage
			return "", errors.New(errorMessage)
		}
		return "", err
	}

	return loadBalancerIngress, nil
}

// GetLoadBalancerIngress takes a context, a client, a service object. It gets the `service` and
// queries for a load balancer's technical name (ip address or hostname). It returns the value of the technical name
// whereby it always prefers the hostname (if given) over the IP address.
// The passed `service` instance is updated with the information received from the API server.
func GetLoadBalancerIngress(ctx context.Context, c client.Client, service *corev1.Service) (string, error) {
	if err := c.Get(ctx, client.ObjectKeyFromObject(service), service); err != nil {
		return "", err
	}

	var (
		serviceStatusIngress = service.Status.LoadBalancer.Ingress
		length               = len(serviceStatusIngress)
	)

	switch {
	case length == 0:
		return "", errors.New("`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created")
	case serviceStatusIngress[length-1].Hostname != "":
		return serviceStatusIngress[length-1].Hostname, nil
	case serviceStatusIngress[length-1].IP != "":
		return serviceStatusIngress[length-1].IP, nil
	}

	return "", errors.New("`.status.loadBalancer.ingress[]` has an element which does neither contain `.ip` nor `.hostname`")
}

// LookupObject retrieves an obj for the given object key dealing with potential stale cache that still does not contain the obj.
// It first tries to retrieve the obj using the given cached client.
// If the object key is not found, then it does live lookup from the API server using the given apiReader.
func LookupObject(ctx context.Context, c client.Client, apiReader client.Reader, key client.ObjectKey, obj client.Object) error {
	err := c.Get(ctx, key, obj)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	// Try to get the obj, now by doing a live lookup instead of relying on the cache.
	return apiReader.Get(ctx, key, obj)
}

// FeatureGatesToCommandLineParameter transforms feature gates given as string/bool map to a command line parameter that
// is understood by Kubernetes components.
func FeatureGatesToCommandLineParameter(fg map[string]bool) string {
	if len(fg) == 0 {
		return ""
	}

	keys := make([]string, 0, len(fg))
	for k := range fg {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := "--feature-gates="
	for _, key := range keys {
		out += fmt.Sprintf("%s=%s,", key, strconv.FormatBool(fg[key]))
	}
	return out
}

// ReconcileServicePorts reconciles the existing service ports with the desired ports. This means that it takes the
// existing port (identified by name), and applies the settings from the desired port to it. This way it can keep fields
// that are defaulted by controllers, e.g. the node port. However, it does not keep ports that are not part of the
// desired list.
func ReconcileServicePorts(existingPorts []corev1.ServicePort, desiredPorts []corev1.ServicePort) []corev1.ServicePort {
	var out []corev1.ServicePort

	for _, desiredPort := range desiredPorts {
		var port corev1.ServicePort

		for _, existingPort := range existingPorts {
			if existingPort.Name == desiredPort.Name {
				port = existingPort
				break
			}
		}

		port.Name = desiredPort.Name
		if len(desiredPort.Protocol) > 0 {
			port.Protocol = desiredPort.Protocol
		}
		if desiredPort.Port != 0 {
			port.Port = desiredPort.Port
		}
		if desiredPort.TargetPort.Type == intstr.Int || desiredPort.TargetPort.Type == intstr.String {
			port.TargetPort = desiredPort.TargetPort
		}
		if desiredPort.NodePort != 0 {
			port.NodePort = desiredPort.NodePort
		}

		out = append(out, port)
	}

	return out
}

// FetchEventMessages gets events for the given object of the given `eventType` and returns them as a formatted output.
// The function expects that the given `obj` is specified with a proper `metav1.TypeMeta`.
func FetchEventMessages(ctx context.Context, c client.Client, obj client.Object, eventType string, eventsLimit int) (string, error) {
	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	if apiVersion == "" {
		return "", fmt.Errorf("apiVersion not specified for object %s/%s", obj.GetNamespace(), obj.GetName())
	}
	if kind == "" {
		return "", fmt.Errorf("kind not specified for object %s/%s", obj.GetNamespace(), obj.GetName())
	}
	fieldSelector := client.MatchingFields{
		"involvedObject.apiVersion": apiVersion,
		"involvedObject.kind":       kind,
		"involvedObject.name":       obj.GetName(),
		"involvedObject.namespace":  obj.GetNamespace(),
		"type":                      eventType,
	}
	eventList := &corev1.EventList{}
	if err := c.List(ctx, eventList, fieldSelector); err != nil {
		return "", fmt.Errorf("error '%v' occurred while fetching more details", err)
	}

	if len(eventList.Items) > 0 {
		return buildEventsErrorMessage(eventList.Items, eventsLimit), nil
	}
	return "", nil
}

func buildEventsErrorMessage(events []corev1.Event, eventsLimit int) string {
	sortByLastTimestamp := func(o1, o2 client.Object) bool {
		obj1, ok1 := o1.(*corev1.Event)
		obj2, ok2 := o2.(*corev1.Event)

		if !ok1 || !ok2 {
			return false
		}

		return obj1.LastTimestamp.Time.Before(obj2.LastTimestamp.Time)
	}

	list := &corev1.EventList{Items: events}
	SortBy(sortByLastTimestamp).Sort(list)
	events = list.Items

	if len(events) > eventsLimit {
		events = events[len(events)-eventsLimit:]
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "-> Events:")
	for _, event := range events {
		var interval string
		if event.Count > 1 {
			interval = fmt.Sprintf("%s ago (x%d over %s)", translateTimestampSince(event.LastTimestamp), event.Count, translateTimestampSince(event.FirstTimestamp))
		} else {
			interval = fmt.Sprintf("%s ago", translateTimestampSince(event.FirstTimestamp))
			if event.FirstTimestamp.IsZero() {
				interval = fmt.Sprintf("%s ago", translateMicroTimestampSince(event.EventTime))
			}
		}
		source := event.Source.Component
		if source == "" {
			source = event.ReportingController
		}

		fmt.Fprintf(&builder, "\n* %s reported %s: %s", source, interval, event.Message)
	}

	return builder.String()
}

// translateTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateTimestampSince(timestamp metav1.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

// translateMicroTimestampSince returns the elapsed time since timestamp in
// human-readable approximation.
func translateMicroTimestampSince(timestamp metav1.MicroTime) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}

	return duration.HumanDuration(time.Since(timestamp.Time))
}

// MergeOwnerReferences merges the newReferences with the list of existing references.
func MergeOwnerReferences(references []metav1.OwnerReference, newReferences ...metav1.OwnerReference) []metav1.OwnerReference {
	uids := make(map[types.UID]struct{})
	for _, reference := range references {
		uids[reference.UID] = struct{}{}
	}

	for _, newReference := range newReferences {
		if _, ok := uids[newReference.UID]; !ok {
			references = append(references, newReference)
		}
	}

	return references
}

// OwnedBy checks if the given object's owner reference contains an entry with the provided attributes.
func OwnedBy(obj runtime.Object, apiVersion, kind, name string, uid types.UID) bool {
	acc, err := meta.Accessor(obj)
	if err != nil {
		return false
	}

	for _, ownerReference := range acc.GetOwnerReferences() {
		return ownerReference.APIVersion == apiVersion &&
			ownerReference.Kind == kind &&
			ownerReference.Name == name &&
			ownerReference.UID == uid
	}

	return false
}

// NewestObject returns the most recently created object based on the provided list object type. If a filter function
// is provided then it will be applied for each object right after listing all objects. If no object remains then nil
// is returned. The Items field in the list object will be populated with the result returned from the server after
// applying the filter function (if provided).
func NewestObject(ctx context.Context, c client.Client, listObj client.ObjectList, filterFn func(runtime.Object) bool, listOpts ...client.ListOption) (runtime.Object, error) {
	if err := c.List(ctx, listObj, listOpts...); err != nil {
		return nil, err
	}

	if filterFn != nil {
		var items []runtime.Object

		if err := meta.EachListItem(listObj, func(obj runtime.Object) error {
			if filterFn(obj) {
				items = append(items, obj)
			}
			return nil
		}); err != nil {
			return nil, err
		}

		if err := meta.SetList(listObj, items); err != nil {
			return nil, err
		}
	}

	if meta.LenList(listObj) == 0 {
		return nil, nil
	}

	ByCreationTimestamp().Sort(listObj)

	items, err := meta.ExtractList(listObj)
	if err != nil {
		return nil, err
	}

	return items[meta.LenList(listObj)-1], nil
}

// NewestPodForDeployment returns the most recently created Pod object for the given deployment.
func NewestPodForDeployment(ctx context.Context, c client.Client, deployment *appsv1.Deployment) (*corev1.Pod, error) {
	listOpts := []client.ListOption{client.InNamespace(deployment.Namespace)}
	if deployment.Spec.Selector != nil {
		listOpts = append(listOpts, client.MatchingLabels(deployment.Spec.Selector.MatchLabels))
	}

	replicaSet, err := NewestObject(
		ctx,
		c,
		&appsv1.ReplicaSetList{},
		func(obj runtime.Object) bool {
			return OwnedBy(obj, appsv1.SchemeGroupVersion.String(), "Deployment", deployment.Name, deployment.UID)
		},
		listOpts...,
	)
	if err != nil {
		return nil, err
	}
	if replicaSet == nil {
		return nil, nil
	}

	newestReplicaSet, ok := replicaSet.(*appsv1.ReplicaSet)
	if !ok {
		return nil, fmt.Errorf("object is not of type *appsv1.ReplicaSet but %T", replicaSet)
	}

	pod, err := NewestObject(
		ctx,
		c,
		&corev1.PodList{},
		func(obj runtime.Object) bool {
			return OwnedBy(obj, appsv1.SchemeGroupVersion.String(), "ReplicaSet", newestReplicaSet.Name, newestReplicaSet.UID)
		},
		listOpts...,
	)
	if err != nil {
		return nil, err
	}
	if pod == nil {
		return nil, nil
	}

	newestPod, ok := pod.(*corev1.Pod)
	if !ok {
		return nil, fmt.Errorf("object is not of type *corev1.Pod but %T", pod)
	}

	return newestPod, nil
}

// MostRecentCompleteLogs returns the logs of the pod/container in case it is not running. If the pod/container is
// running then the logs of the previous pod/container are being returned.
func MostRecentCompleteLogs(
	ctx context.Context,
	podInterface corev1client.PodInterface,
	pod *corev1.Pod,
	containerName string,
	tailLines *int64,
) (
	string,
	error,
) {
	previousLogs := false
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerName == "" || containerStatus.Name == containerName {
			previousLogs = containerStatus.State.Running != nil
			break
		}
	}

	logs, err := kubernetes.GetPodLogs(ctx, podInterface, pod.Name, &corev1.PodLogOptions{
		Container: containerName,
		TailLines: tailLines,
		Previous:  previousLogs,
	})
	if err != nil {
		return "", err
	}

	return string(logs), nil
}
