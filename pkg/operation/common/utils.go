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

package common

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"

	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	autoscalingv1beta2 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
	"k8s.io/client-go/util/retry"
	"k8s.io/component-base/version"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// GetSecretKeysWithPrefix returns a list of keys of the given map <m> which are prefixed with <kind>.
func GetSecretKeysWithPrefix(kind string, m map[string]*corev1.Secret) []string {
	var result []string
	for key := range m {
		if strings.HasPrefix(key, kind) {
			result = append(result, key)
		}
	}
	return result
}

// ComputeOffsetIP parses the provided <subnet> and offsets with the value of <offset>.
// For example, <subnet> = 100.64.0.0/11 and <offset> = 10 the result would be 100.64.0.10
// IPv6 and IPv4 is supported.
func ComputeOffsetIP(subnet *net.IPNet, offset int64) (net.IP, error) {
	if subnet == nil {
		return nil, fmt.Errorf("subnet is nil")
	}

	isIPv6 := false

	bytes := subnet.IP.To4()
	if bytes == nil {
		isIPv6 = true
		bytes = subnet.IP.To16()
	}

	ip := net.IP(big.NewInt(0).Add(big.NewInt(0).SetBytes(bytes), big.NewInt(offset)).Bytes())

	if !subnet.Contains(ip) {
		return nil, fmt.Errorf("cannot compute IP with offset %d - subnet %q too small", offset, subnet)
	}

	// there is no broadcast address on IPv6
	if isIPv6 {
		return ip, nil
	}

	for i := range ip {
		// IP address is not the same, so it's not the broadcast ip.
		if ip[i] != ip[i]|^subnet.Mask[i] {
			return ip.To4(), nil
		}
	}

	return nil, fmt.Errorf("computed IPv4 address %q is broadcast for subnet %q", ip, subnet)
}

// GenerateAddonConfig returns the provided <values> in case <enabled> is true. Otherwise, nil is
// being returned.
func GenerateAddonConfig(values map[string]interface{}, enabled bool) map[string]interface{} {
	v := map[string]interface{}{
		"enabled": enabled,
	}
	if enabled {
		for key, value := range values {
			v[key] = value
		}
	}
	return v
}

// GenerateBackupEntryName returns BackupEntry resource name created from provided <seedNamespace> and <shootUID>.
func GenerateBackupEntryName(seedNamespace string, shootUID types.UID) string {
	return fmt.Sprintf("%s--%s", seedNamespace, shootUID)
}

// ExtractShootDetailsFromBackupEntryName returns Shoot resource technicalID its UID from provided <backupEntryName>.
func ExtractShootDetailsFromBackupEntryName(backupEntryName string) (shootTechnicalID, shootUID string) {
	tokens := strings.Split(backupEntryName, "--")
	shootUID = tokens[len(tokens)-1]
	shootTechnicalID = strings.TrimSuffix(backupEntryName, shootUID)
	shootTechnicalID = strings.TrimSuffix(shootTechnicalID, "--")
	return shootTechnicalID, shootUID
}

// IsFollowingNewNamingConvention determines whether the new naming convention followed for shoot resources.
// TODO: Remove this and use only "--" as separator, once we have all shoots deployed as per new naming conventions.
func IsFollowingNewNamingConvention(seedNamespace string) bool {
	return len(strings.Split(seedNamespace, "--")) > 2
}

// ReplaceCloudProviderConfigKey replaces a key with the new value in the given cloud provider config.
func ReplaceCloudProviderConfigKey(cloudProviderConfig, separator, key, value string) string {
	keyValueRegexp := regexp.MustCompile(fmt.Sprintf(`(\Q%s\E%s)([^\n]*)`, key, separator))
	return keyValueRegexp.ReplaceAllString(cloudProviderConfig, fmt.Sprintf(`${1}%q`, strings.Replace(value, `$`, `$$`, -1)))
}

// ProjectForNamespace returns the project object responsible for a given <namespace>.
// It tries to identify the project object by looking for the namespace name in the project spec.
func ProjectForNamespace(projectLister gardencorelisters.ProjectLister, namespaceName string) (*gardencorev1beta1.Project, error) {
	projectList, err := projectLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var projects []gardencorev1beta1.Project
	for _, p := range projectList {
		projects = append(projects, *p)
	}

	return projectForNamespace(projects, namespaceName)
}

// ProjectForNamespaceWithClient returns the project object responsible for a given <namespace>.
// It tries to identify the project object by looking for the namespace name in the project spec.
func ProjectForNamespaceWithClient(ctx context.Context, c client.Client, namespaceName string) (*gardencorev1beta1.Project, error) {
	projectList := &gardencorev1beta1.ProjectList{}
	err := c.List(ctx, projectList)
	if err != nil {
		return nil, err
	}

	return projectForNamespace(projectList.Items, namespaceName)
}

func projectForNamespace(projects []gardencorev1beta1.Project, namespaceName string) (*gardencorev1beta1.Project, error) {
	for _, project := range projects {
		if project.Spec.Namespace != nil && *project.Spec.Namespace == namespaceName {
			return &project, nil
		}
	}

	return nil, apierrors.NewNotFound(gardencorev1beta1.Resource("Project"), fmt.Sprintf("for namespace %s", namespaceName))
}

// ProjectNameForNamespace determines the project name for a given <namespace>. It tries to identify it first per the namespace's ownerReferences.
// If it doesn't help then it will check whether the project name is a label on the namespace object. If it doesn't help then the name can be inferred
// from the namespace name in case it is prefixed with the project prefix. If none of those approaches the namespace name itself is returned as project
// name.
func ProjectNameForNamespace(namespace *corev1.Namespace) string {
	for _, ownerReference := range namespace.OwnerReferences {
		if ownerReference.Kind == "Project" {
			return ownerReference.Name
		}
	}

	if name, ok := namespace.Labels[ProjectName]; ok {
		return name
	}

	if nameSplit := strings.Split(namespace.Name, ProjectPrefix); len(nameSplit) > 1 {
		return nameSplit[1]
	}

	return namespace.Name
}

// GardenerDeletionGracePeriod is the default grace period for Gardener's force deletion methods.
var GardenerDeletionGracePeriod = 5 * time.Minute

// ShouldObjectBeRemoved determines whether the given object should be gone now.
// This is calculated by first checking the deletion timestamp of an object: If the deletion timestamp
// is unset, the object should not be removed - i.e. this returns false.
// Otherwise, it is checked whether the deletionTimestamp is before the current time minus the
// grace period.
func ShouldObjectBeRemoved(obj metav1.Object, gracePeriod time.Duration) bool {
	deletionTimestamp := obj.GetDeletionTimestamp()
	if deletionTimestamp == nil {
		return false
	}

	return deletionTimestamp.Time.Before(time.Now().Add(-gracePeriod))
}

// DeleteHvpa delete all resources required for the HVPA in the given namespace.
func DeleteHvpa(ctx context.Context, k8sClient kubernetes.Interface, namespace string) error {
	if k8sClient == nil {
		return fmt.Errorf("require kubernetes client")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", v1beta1constants.GardenRole, GardenRoleHvpa),
	}

	// Delete all CRDs with label "gardener.cloud/role=hvpa"
	// Workaround: Due to https://github.com/gardener/gardener/issues/2257, we first list the HVPA CRDs and then remove
	// them one by one.
	crdList, err := k8sClient.APIExtension().ApiextensionsV1beta1().CustomResourceDefinitions().List(ctx, listOptions)
	if err != nil {
		return err
	}
	for _, crd := range crdList.Items {
		if err := k8sClient.APIExtension().ApiextensionsV1beta1().CustomResourceDefinitions().Delete(ctx, crd.Name, metav1.DeleteOptions{}); client.IgnoreNotFound(err) != nil {
			return err
		}
	}

	// Delete all Deployments with label "gardener.cloud/role=hvpa"
	deletePropagation := metav1.DeletePropagationForeground
	if err := k8sClient.Kubernetes().AppsV1().Deployments(namespace).DeleteCollection(ctx, metav1.DeleteOptions{PropagationPolicy: &deletePropagation}, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Delete all ClusterRoles with label "gardener.cloud/role=hvpa"
	if err := k8sClient.Kubernetes().RbacV1().ClusterRoles().DeleteCollection(ctx, metav1.DeleteOptions{}, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Delete all ClusterRoleBindings with label "gardener.cloud/role=hvpa"
	if err := k8sClient.Kubernetes().RbacV1().ClusterRoleBindings().DeleteCollection(ctx, metav1.DeleteOptions{}, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}

	// Delete all ServiceAccounts with label "gardener.cloud/role=hvpa"
	if err := k8sClient.Kubernetes().CoreV1().ServiceAccounts(namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}

	return nil
}

// DeleteVpa delete all resources required for the VPA in the given namespace.
func DeleteVpa(ctx context.Context, c client.Client, namespace string, isShoot bool) error {
	resources := []client.Object{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPAAdmissionController, Namespace: namespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPARecommender, Namespace: namespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPAUpdater, Namespace: namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "vpa-webhook", Namespace: namespace}},
		&autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPAAdmissionController, Namespace: namespace}},
		&autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPARecommender, Namespace: namespace}},
		&autoscalingv1beta2.VerticalPodAutoscaler{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameVPAUpdater, Namespace: namespace}},
	}

	if isShoot {
		resources = append(resources,
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpa-admission-controller", Namespace: namespace}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpa-recommender", Namespace: namespace}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: VPASecretName, Namespace: namespace}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "vpa-updater", Namespace: namespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-kube-apiserver-to-vpa-admission-controller", Namespace: namespace}},
		)
	} else {
		resources = append(resources,
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:actor"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:admission-controller"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:checkpoint-actor"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:metrics-reader"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:target-reader"}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:actor"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:admission-controller"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:checkpoint-actor"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:metrics-reader"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:target-reader"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "gardener.cloud:vpa:seed:evictioner"}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "vpa-admission-controller", Namespace: namespace}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "vpa-recommender", Namespace: namespace}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "vpa-updater", Namespace: namespace}},
			&admissionregistrationv1beta1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: "vpa-webhook-config-seed"}},
		)
	}

	for _, resource := range resources {
		if err := c.Delete(ctx, resource); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

// DeleteShootLoggingStack deletes all shoot resource of the logging stack in the given namespace.
func DeleteShootLoggingStack(ctx context.Context, k8sClient client.Client, namespace string) error {
	return DeleteLoki(ctx, k8sClient, namespace)
}

// DeleteLoki  deletes all resources of the Loki in a given namespace.
func DeleteLoki(ctx context.Context, k8sClient client.Client, namespace string) error {
	resources := []client.Object{
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: namespace}},
		&hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "loki-config", Namespace: namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: namespace}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: namespace}},
	}

	for _, resource := range resources {
		if err := k8sClient.Delete(ctx, resource); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
			return err
		}
	}
	return nil
}

// DeleteSeedLoggingStack deletes all seed resource of the logging stack in the garden namespace.
func DeleteSeedLoggingStack(ctx context.Context, k8sClient client.Client) error {
	resources := []client.Object{
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-config", Namespace: v1beta1constants.GardenNamespace}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-fluentbit", Namespace: v1beta1constants.GardenNamespace}},
		&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit"}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
	}

	for _, resource := range resources {
		if err := k8sClient.Delete(ctx, resource); client.IgnoreNotFound(err) != nil && !meta.IsNoMatchError(err) {
			return err
		}
	}

	return DeleteLoki(ctx, k8sClient, v1beta1constants.GardenNamespace)
}

// GetContainerResourcesInStatefulSet  returns the containers resources in StatefulSet
func GetContainerResourcesInStatefulSet(ctx context.Context, k8sClient client.Client, key client.ObjectKey) ([]*corev1.ResourceRequirements, error) {
	statefulSet := &appsv1.StatefulSet{}
	resourcesPerContainer := make([]*corev1.ResourceRequirements, 0)
	if err := k8sClient.Get(ctx, key, statefulSet); client.IgnoreNotFound(err) != nil {
		return nil, err
	} else if !apierrors.IsNotFound(err) {
		for _, container := range statefulSet.Spec.Template.Spec.Containers {
			resourcesPerContainer = append(resourcesPerContainer, container.Resources.DeepCopy())
		}
		return resourcesPerContainer, nil
	}

	// Use the default resources defined in values file
	return nil, nil
}

// DeleteReserveExcessCapacity deletes the deployment and priority class for excess capacity
func DeleteReserveExcessCapacity(ctx context.Context, k8sClient client.Client) error {
	if k8sClient == nil {
		return errors.New("must provide non-nil kubernetes client to common.DeleteReserveExcessCapacity")
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "reserve-excess-capacity",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
	if err := k8sClient.Delete(ctx, deploy); client.IgnoreNotFound(err) != nil {
		return err
	}

	priorityClass := &schedulingv1beta1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gardener-reserve-excess-capacity",
		},
	}
	return client.IgnoreNotFound(k8sClient.Delete(ctx, priorityClass))
}

// DeleteAlertmanager deletes all resources of the Alertmanager in a given namespace.
func DeleteAlertmanager(ctx context.Context, k8sClient client.Client, namespace string) error {
	objs := []client.Object{
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      v1beta1constants.StatefulSetNameAlertManager,
				Namespace: namespace,
			},
		},
		&extensionsv1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager",
				Namespace: namespace,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-client",
				Namespace: namespace,
			},
		},
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager",
				Namespace: namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-basic-auth",
				Namespace: namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      AlertManagerTLS,
				Namespace: namespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-config",
				Namespace: namespace,
			},
		},
		&corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-db-alertmanager-0",
				Namespace: namespace,
			},
		},
	}

	return kutil.DeleteObjects(ctx, k8sClient, objs...)
}

// DeleteGrafanaByRole deletes the monitoring stack for the shoot owner.
func DeleteGrafanaByRole(ctx context.Context, k8sClient kubernetes.Interface, namespace, role string) error {
	if k8sClient == nil {
		return fmt.Errorf("require kubernetes client")
	}

	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s,%s=%s", "component", "grafana", "role", role),
	}

	deletePropagation := metav1.DeletePropagationForeground
	if err := k8sClient.Kubernetes().AppsV1().Deployments(namespace).DeleteCollection(
		ctx,
		metav1.DeleteOptions{
			PropagationPolicy: &deletePropagation,
		}, listOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := k8sClient.Kubernetes().CoreV1().ConfigMaps(namespace).DeleteCollection(
		ctx, metav1.DeleteOptions{}, listOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := k8sClient.Kubernetes().ExtensionsV1beta1().Ingresses(namespace).DeleteCollection(
		ctx, metav1.DeleteOptions{}, listOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := k8sClient.Kubernetes().CoreV1().Secrets(namespace).DeleteCollection(
		ctx, metav1.DeleteOptions{}, listOptions); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if err := k8sClient.Kubernetes().CoreV1().Services(namespace).Delete(
		ctx, fmt.Sprintf("grafana-%s", role), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

// GetDomainInfoFromAnnotations returns the provider and the domain that is specified in the give annotations.
func GetDomainInfoFromAnnotations(annotations map[string]string) (provider string, domain string, includeZones, excludeZones []string, err error) {
	if annotations == nil {
		return "", "", nil, nil, fmt.Errorf("domain secret has no annotations")
	}

	if providerAnnotation, ok := annotations[DNSProvider]; ok {
		provider = providerAnnotation
	}

	if domainAnnotation, ok := annotations[DNSDomain]; ok {
		domain = domainAnnotation
	}

	if includeZonesAnnotation, ok := annotations[DNSIncludeZones]; ok {
		includeZones = strings.Split(includeZonesAnnotation, ",")
	}
	if excludeZonesAnnotation, ok := annotations[DNSExcludeZones]; ok {
		excludeZones = strings.Split(excludeZonesAnnotation, ",")
	}

	if len(domain) == 0 {
		return "", "", nil, nil, fmt.Errorf("missing dns domain annotation on domain secret")
	}
	if len(provider) == 0 {
		return "", "", nil, nil, fmt.Errorf("missing dns provider annotation on domain secret")
	}

	return
}

// CurrentReplicaCount returns the current replicaCount for the given deployment.
func CurrentReplicaCount(ctx context.Context, client client.Client, namespace, deploymentName string) (int32, error) {
	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, kutil.Key(namespace, deploymentName), deployment); err != nil && !apierrors.IsNotFound(err) {
		return 0, err
	}
	if deployment.Spec.Replicas == nil {
		return 0, nil
	}
	return *deployment.Spec.Replicas, nil
}

// RespectShootSyncPeriodOverwrite checks whether to respect the sync period overwrite of a Shoot or not.
func RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot) bool {
	return respectSyncPeriodOverwrite || shoot.Namespace == v1beta1constants.GardenNamespace
}

// ShouldIgnoreShoot determines whether a Shoot should be ignored or not.
func ShouldIgnoreShoot(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot) bool {
	if !RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot) {
		return false
	}

	value, ok := shoot.Annotations[ShootIgnore]
	if !ok {
		return false
	}

	ignore, _ := strconv.ParseBool(value)
	return ignore
}

// IsShootFailed checks if a Shoot is failed.
func IsShootFailed(shoot *gardencorev1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation

	return lastOperation != nil && lastOperation.State == gardencorev1beta1.LastOperationStateFailed &&
		shoot.Generation == shoot.Status.ObservedGeneration &&
		shoot.Status.Gardener.Version == version.Get().GitVersion
}

// IsNowInEffectiveShootMaintenanceTimeWindow checks if the current time is in the effective
// maintenance time window of the Shoot.
func IsNowInEffectiveShootMaintenanceTimeWindow(shoot *gardencorev1beta1.Shoot) bool {
	return EffectiveShootMaintenanceTimeWindow(shoot).Contains(time.Now())
}

// LastReconciliationDuringThisTimeWindow returns true if <now> is contained in the given effective maintenance time
// window of the shoot and if the <lastReconciliation> did not happen longer than the longest possible duration of a
// maintenance time window.
func LastReconciliationDuringThisTimeWindow(shoot *gardencorev1beta1.Shoot) bool {
	if shoot.Status.LastOperation == nil {
		return false
	}

	var (
		timeWindow         = EffectiveShootMaintenanceTimeWindow(shoot)
		now                = time.Now()
		lastReconciliation = shoot.Status.LastOperation.LastUpdateTime.Time
	)

	return timeWindow.Contains(lastReconciliation) && now.UTC().Sub(lastReconciliation.UTC()) <= gardencorev1beta1.MaintenanceTimeWindowDurationMaximum
}

// IsObservedAtLatestGenerationAndSucceeded checks whether the Shoot's generation has changed or if the LastOperation status
// is Succeeded.
func IsObservedAtLatestGenerationAndSucceeded(shoot *gardencorev1beta1.Shoot) bool {
	lastOperation := shoot.Status.LastOperation
	return shoot.Generation == shoot.Status.ObservedGeneration &&
		(lastOperation != nil && lastOperation.State == gardencorev1beta1.LastOperationStateSucceeded)
}

// SyncPeriodOfShoot determines the sync period of the given shoot.
//
// If no overwrite is allowed, the defaultMinSyncPeriod is returned.
// Otherwise, the overwrite is parsed. If an error occurs or it is smaller than the defaultMinSyncPeriod,
// the defaultMinSyncPeriod is returned. Otherwise, the overwrite is returned.
func SyncPeriodOfShoot(respectSyncPeriodOverwrite bool, defaultMinSyncPeriod time.Duration, shoot *gardencorev1beta1.Shoot) time.Duration {
	if !RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot) {
		return defaultMinSyncPeriod
	}

	syncPeriodOverwrite, ok := shoot.Annotations[ShootSyncPeriod]
	if !ok {
		return defaultMinSyncPeriod
	}

	syncPeriod, err := time.ParseDuration(syncPeriodOverwrite)
	if err != nil {
		return defaultMinSyncPeriod
	}

	if syncPeriod < defaultMinSyncPeriod {
		return defaultMinSyncPeriod
	}
	return syncPeriod
}

// EffectiveMaintenanceTimeWindow cuts a maintenance time window at the end with a guess of 15 minutes. It is subtracted from the end
// of a maintenance time window to use a best-effort kind of finishing the operation before the end.
// Generally, we can't make sure that the maintenance operation is done by the end of the time window anyway (considering large
// clusters with hundreds of nodes, a rolling update will take several hours).
func EffectiveMaintenanceTimeWindow(timeWindow *utils.MaintenanceTimeWindow) *utils.MaintenanceTimeWindow {
	return timeWindow.WithEnd(timeWindow.End().Add(0, -15, 0))
}

// EffectiveShootMaintenanceTimeWindow returns the effective MaintenanceTimeWindow of the given Shoot.
func EffectiveShootMaintenanceTimeWindow(shoot *gardencorev1beta1.Shoot) *utils.MaintenanceTimeWindow {
	maintenance := shoot.Spec.Maintenance
	if maintenance == nil || maintenance.TimeWindow == nil {
		return utils.AlwaysTimeWindow
	}

	timeWindow, err := utils.ParseMaintenanceTimeWindow(maintenance.TimeWindow.Begin, maintenance.TimeWindow.End)
	if err != nil {
		return utils.AlwaysTimeWindow
	}

	return EffectiveMaintenanceTimeWindow(timeWindow)
}

// GardenEtcdEncryptionSecretName returns the name to the 'backup' of the etcd encryption secret in the Garden cluster.
func GardenEtcdEncryptionSecretName(shootName string) string {
	return fmt.Sprintf("%s.%s", shootName, EtcdEncryptionSecretName)
}

// ReadServiceAccountSigningKeySecret reads the signing key secret to extract the signing key.
// It errors if there is no value at ServiceAccountSigningKeySecretDataKey.
func ReadServiceAccountSigningKeySecret(secret *corev1.Secret) (string, error) {
	data, ok := secret.Data[ServiceAccountSigningKeySecretDataKey]
	if !ok {
		return "", fmt.Errorf("no signing key secret in secret %s/%s at .Data.%s", secret.Namespace, secret.Name, ServiceAccountSigningKeySecretDataKey)
	}

	return string(data), nil
}

// GetServiceAccountSigningKeySecret gets the signing key from the secret with the given name and namespace.
func GetServiceAccountSigningKeySecret(ctx context.Context, c client.Client, shootNamespace, secretName string) (string, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, kutil.Key(shootNamespace, secretName), secret); err != nil {
		return "", err
	}

	return ReadServiceAccountSigningKeySecret(secret)
}

// GetAPIServerDomain returns the fully qualified domain name of for the api-server for the Shoot cluster. The
// end result is 'api.<domain>'.
func GetAPIServerDomain(domain string) string {
	return fmt.Sprintf("%s.%s", APIServerPrefix, domain)
}

// GetSecretFromSecretRef gets the Secret object from <secretRef>.
func GetSecretFromSecretRef(ctx context.Context, c client.Client, secretRef *corev1.SecretReference) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := c.Get(ctx, kutil.Key(secretRef.Namespace, secretRef.Name), secret); err != nil {
		return nil, err
	}
	return secret, nil
}

// CheckIfDeletionIsConfirmed returns whether the deletion of an object is confirmed or not.
func CheckIfDeletionIsConfirmed(obj metav1.Object) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return annotationRequiredError()
	}

	value := annotations[ConfirmationDeletion]
	if confirmed, err := strconv.ParseBool(value); err != nil || !confirmed {
		return annotationRequiredError()
	}
	return nil
}

func annotationRequiredError() error {
	return fmt.Errorf("must have a %q annotation to delete", ConfirmationDeletion)
}

// ConfirmDeletion adds Gardener's deletion confirmation annotation to the given object and sends an UPDATE request.
func ConfirmDeletion(ctx context.Context, c client.Client, obj client.Object) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		if err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			return nil
		}

		existing := obj.DeepCopyObject()

		acc, err := meta.Accessor(obj)
		if err != nil {
			return err
		}
		kutil.SetMetaDataAnnotation(acc, ConfirmationDeletion, "true")
		kutil.SetMetaDataAnnotation(acc, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		if reflect.DeepEqual(existing, obj) {
			return nil
		}

		return c.Update(ctx, obj)
	})
}

// ExtensionID returns an identifier for the given extension kind/type.
func ExtensionID(extensionKind, extensionType string) string {
	return fmt.Sprintf("%s/%s", extensionKind, extensionType)
}

// DeleteDeploymentsHavingDeprecatedRoleLabelKey deletes the Deployments with the passed object keys if
// the corresponding Deployment .spec.selector contains the deprecated "garden.sapcloud.io/role" label key.
func DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx context.Context, c client.Client, keys []client.ObjectKey) error {
	for _, key := range keys {
		deployment := &appsv1.Deployment{}
		if err := c.Get(ctx, key, deployment); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			return err
		}

		if _, ok := deployment.Spec.Selector.MatchLabels[v1beta1constants.DeprecatedGardenRole]; ok {
			if err := c.Delete(ctx, deployment); client.IgnoreNotFound(err) != nil {
				return err
			}

			if err := kutil.WaitUntilResourceDeleted(ctx, c, deployment, 2*time.Second); err != nil {
				return err
			}
		}
	}

	return nil
}
