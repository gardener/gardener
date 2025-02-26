// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
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
	"k8s.io/client-go/rest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/gardener/gardener/pkg/utils/retry"
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

// SetAnnotationAndUpdate sets the annotation on the given object and updates it.
func SetAnnotationAndUpdate(ctx context.Context, c client.Client, obj client.Object, key, value string) error {
	if !HasMetaDataAnnotation(obj, key, value) {
		objCopy := obj.DeepCopyObject().(client.Object)
		SetMetaDataAnnotation(obj, key, value)
		return c.Patch(ctx, obj, client.MergeFrom(objCopy))
	}
	return nil
}

// ObjectKeyFromSecretRef returns an ObjectKey for the given SecretReference.
func ObjectKeyFromSecretRef(ref corev1.SecretReference) client.ObjectKey {
	return client.ObjectKey{
		Namespace: ref.Namespace,
		Name:      ref.Name,
	}
}

// WaitUntilResourceDeleted waits until it has been deleted. It respects the given interval. Timeout must be provided
// via the context.
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
func WaitUntilLoadBalancerIsReady(
	ctx context.Context,
	log logr.Logger,
	c client.Client,
	namespace, name string,
	timeout time.Duration,
) (
	string,
	error,
) {
	var (
		loadBalancerIngress string
		service             = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	)

	log = log.WithValues("service", client.ObjectKeyFromObject(service))

	if err := retry.UntilTimeout(ctx, 5*time.Second, timeout, func(ctx context.Context) (done bool, err error) {
		loadBalancerIngress, err = GetLoadBalancerIngress(ctx, c, service)
		if err != nil {
			log.Info("Waiting until service is ready")
			return retry.MinorError(fmt.Errorf("%s service is not ready: %v", name, err))
		}
		return retry.Ok()
	}); err != nil {
		log.Error(err, "Error while waiting for load balancer to be ready")

		// use API reader here, we don't want to cache all events
		eventsErrorMessage, err2 := FetchEventMessages(ctx, c.Scheme(), c, service, corev1.EventTypeWarning, 2)
		if err2 != nil {
			log.Error(err2, "Error while fetching events for load balancer service")
			return "", fmt.Errorf("'%w' occurred but could not fetch events for more information", err)
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
	return MapStringBoolToCommandLineParameter(fg, "--feature-gates=")
}

// MapStringBoolToCommandLineParameter transforms a string/bool map to a command line parameter that is understood by
// Kubernetes components.
func MapStringBoolToCommandLineParameter(m map[string]bool, param string) string {
	if len(m) == 0 {
		return ""
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	out := param
	for _, key := range keys {
		out += fmt.Sprintf("%s=%s,", key, strconv.FormatBool(m[key]))
	}
	return strings.TrimSuffix(out, ",")
}

// ReconcileServicePorts reconciles the existing service ports with the desired ports. This means that it takes the
// existing port (identified by name), and applies the settings from the desired port to it. This way it can keep fields
// that are defaulted by controllers, e.g. the node port. However, it does not keep ports that are not part of the
// desired list.
func ReconcileServicePorts(existingPorts []corev1.ServicePort, desiredPorts []corev1.ServicePort, desiredServiceType corev1.ServiceType) []corev1.ServicePort {
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

		// If the desired service type is "LoadBalancer" or "NodePort", then overwrite the existing nodePort
		// only when the desired nodePort != 0 (in this way we preserve the value defaulted by the controller).
		// Otherwise, always set the existing nodePort to the desired one.
		switch desiredServiceType {
		case corev1.ServiceTypeLoadBalancer, corev1.ServiceTypeNodePort:
			if desiredPort.NodePort != 0 {
				port.NodePort = desiredPort.NodePort
			}
		default:
			port.NodePort = desiredPort.NodePort
		}

		out = append(out, port)
	}

	return out
}

// FetchEventMessages gets events for the given object of the given `eventType` and returns them as a formatted output.
// The function expects that the given `obj` is specified with a proper `metav1.TypeMeta`.
func FetchEventMessages(ctx context.Context, scheme *runtime.Scheme, reader client.Reader, obj client.Object, eventType string, eventsLimit int) (string, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return "", fmt.Errorf("failed to identify GVK for object: %w", err)
	}

	apiVersion, kind := gvk.ToAPIVersionAndKind()
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
	if err := reader.List(ctx, eventList, fieldSelector, client.InNamespace(obj.GetNamespace())); err != nil {
		return "", fmt.Errorf("error '%w' occurred while fetching more details", err)
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
func OwnedBy(obj client.Object, apiVersion, kind, name string, uid types.UID) bool {
	for _, ownerReference := range obj.GetOwnerReferences() {
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
func NewestObject(ctx context.Context, c client.Reader, listObj client.ObjectList, filterFn func(client.Object) bool, listOpts ...client.ListOption) (client.Object, error) {
	if err := c.List(ctx, listObj, listOpts...); err != nil {
		return nil, err
	}

	if filterFn != nil {
		var items []runtime.Object

		if err := meta.EachListItem(listObj, func(object runtime.Object) error {
			obj, ok := object.(client.Object)
			if !ok {
				return fmt.Errorf("%T does not implement client.Object", object)
			}

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

	newestObject := items[meta.LenList(listObj)-1]
	obj, ok := newestObject.(client.Object)
	if !ok {
		return nil, fmt.Errorf("%T does not implement client.Object", newestObject)
	}

	return obj, nil
}

// NewestPodForDeployment returns the most recently created Pod object for the given deployment.
func NewestPodForDeployment(ctx context.Context, c client.Reader, deployment *appsv1.Deployment) (*corev1.Pod, error) {
	listOpts := []client.ListOption{client.InNamespace(deployment.Namespace)}
	if deployment.Spec.Selector != nil {
		listOpts = append(listOpts, client.MatchingLabels(deployment.Spec.Selector.MatchLabels))
	}

	replicaSet, err := NewestObject(
		ctx,
		c,
		&appsv1.ReplicaSetList{},
		func(obj client.Object) bool {
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

	if newestReplicaSet.Spec.Selector == nil {
		return nil, fmt.Errorf("no pod selector specified in replicaSet %s/%s", newestReplicaSet.Namespace, newestReplicaSet.Name)
	}

	if len(newestReplicaSet.Spec.Selector.MatchLabels)+len(newestReplicaSet.Spec.Selector.MatchExpressions) == 0 {
		return nil, fmt.Errorf("no matchLabels or matchExpressions specified in replicaSet %s/%s", newestReplicaSet.Namespace, newestReplicaSet.Name)
	}

	podSelector, err := metav1.LabelSelectorAsSelector(newestReplicaSet.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("failed to convert the pod selector from ReplicaSet %s/%s: %w", newestReplicaSet.Namespace, newestReplicaSet.Name, err)
	}

	listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: podSelector})

	pod, err := NewestObject(
		ctx,
		c,
		&corev1.PodList{},
		nil,
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
	tailLines,
	headBytes *int64,
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

	lastLogLines, err := GetPodLogs(ctx, podInterface, pod.Name, &corev1.PodLogOptions{
		Container: containerName,
		TailLines: tailLines,
		Previous:  previousLogs,
	})
	if err != nil {
		return "", err
	}

	if headBytes == nil || *headBytes <= 0 {
		return string(lastLogLines), nil
	}

	firstLogLines, err := GetPodLogs(ctx, podInterface, pod.Name, &corev1.PodLogOptions{
		Container:  containerName,
		Previous:   previousLogs,
		LimitBytes: headBytes,
	})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s\n...\n%s", firstLogLines, lastLogLines), nil
}

// NewKubeconfig returns a new kubeconfig structure.
func NewKubeconfig(contextName string, cluster clientcmdv1.Cluster, authInfo clientcmdv1.AuthInfo) *clientcmdv1.Config {
	if !strings.HasPrefix(cluster.Server, "https://") {
		cluster.Server = "https://" + cluster.Server
	}

	return &clientcmdv1.Config{
		CurrentContext: contextName,
		Clusters: []clientcmdv1.NamedCluster{{
			Name:    contextName,
			Cluster: cluster,
		}},
		AuthInfos: []clientcmdv1.NamedAuthInfo{{
			Name:     contextName,
			AuthInfo: authInfo,
		}},
		Contexts: []clientcmdv1.NamedContext{{
			Name: contextName,
			Context: clientcmdv1.Context{
				Cluster:  contextName,
				AuthInfo: contextName,
			},
		}},
	}
}

// ObjectKeyForCreateWebhooks creates an object key for an object handled by webhooks registered for CREATE verbs.
func ObjectKeyForCreateWebhooks(obj client.Object, req admission.Request) client.ObjectKey {
	namespace := obj.GetNamespace()

	// In webhooks the namespace is not always set in objects due to https://github.com/kubernetes/kubernetes/issues/88282,
	// so try to get the namespace information from the request directly, otherwise the object is presumably not namespaced.
	if len(namespace) == 0 && len(req.Namespace) != 0 {
		namespace = req.Namespace
	}

	name := obj.GetName()
	if len(name) == 0 {
		name = obj.GetGenerateName()
	}

	return client.ObjectKey{Namespace: namespace, Name: name}
}

// ClientCertificateFromRESTConfig returns the client certificate used inside a REST config.
func ClientCertificateFromRESTConfig(restConfig *rest.Config) (*tls.Certificate, error) {
	cert, err := tls.X509KeyPair(restConfig.CertData, restConfig.KeyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse X509 certificate: %w", err)
	}

	if len(cert.Certificate) < 1 {
		return nil, errors.New("the X509 certificate is invalid, no cert/key data found")
	}

	certs, err := x509.ParseCertificates(cert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("the X509 certificate bundle cannot be parsed: %w", err)
	}

	if len(certs) < 1 {
		return nil, errors.New("the X509 certificate bundle does not contain exactly one certificate")
	}

	cert.Leaf = certs[0]
	return &cert, nil
}

// TolerationForTaint returns the corresponding toleration for the given taint.
func TolerationForTaint(taint corev1.Taint) corev1.Toleration {
	operator := corev1.TolerationOpEqual
	if taint.Value == "" {
		operator = corev1.TolerationOpExists
	}

	return corev1.Toleration{
		Key:      taint.Key,
		Operator: operator,
		Value:    taint.Value,
		Effect:   taint.Effect,
	}
}

// ComparableTolerations contains information to transform an ordinary 'corev1.Toleration' object to a semantically
// comparable object that is fully compatible with the 'comparable' Golang interface, see https://github.com/golang/go/blob/de6abd78893e91f26337eb399644b7a6bc3ea583/src/builtin/builtin.go#L102.
type ComparableTolerations struct {
	tolerationSeconds map[int64]*int64
}

// Transform takes a toleration object and exchanges the 'TolerationSeconds' pointer if set. The int64 value will
// be the same but pointers will be **reused** for all passed tolerations that have the same underlying toleration seconds value.
func (c *ComparableTolerations) Transform(toleration corev1.Toleration) corev1.Toleration {
	if toleration.TolerationSeconds == nil {
		return toleration
	}

	if c.tolerationSeconds == nil {
		c.tolerationSeconds = make(map[int64]*int64)
	}

	tolerationSeconds := *toleration.TolerationSeconds
	if _, ok := c.tolerationSeconds[tolerationSeconds]; !ok {
		c.tolerationSeconds[tolerationSeconds] = ptr.To(tolerationSeconds)
	}

	toleration.TolerationSeconds = c.tolerationSeconds[tolerationSeconds]
	return toleration
}

// MaximumResourcesFromResourceList merges the given resource lists.
// If the same resource is defined in both lists, the higher quantity is taken into account.
func MaximumResourcesFromResourceList(list1, list2 corev1.ResourceList) corev1.ResourceList {
	resources := maps.Clone(list1)

	for resource, quantity := range list2 {
		if v, ok := resources[resource]; !ok || v.Cmp(quantity) <= 0 {
			resources[resource] = quantity
		}
	}

	return resources
}
