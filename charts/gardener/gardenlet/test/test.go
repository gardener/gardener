// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

// ValidateGardenletChartPriorityClass validates the priority class of the Gardenlet chart.
func ValidateGardenletChartPriorityClass(ctx context.Context, c client.Client) {
	priorityClass := getEmptyPriorityClass()

	Expect(c.Get(
		ctx,
		client.ObjectKey{Name: priorityClass.Name},
		priorityClass,
	)).ToNot(HaveOccurred())
	Expect(priorityClass.GlobalDefault).To(BeFalse())
	Expect(priorityClass.Value).To(Equal(int32(999998950)))
}

func getEmptyPriorityClass() *schedulingv1.PriorityClass {
	return &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1beta1constants.PriorityClassNameSeedSystemCritical,
		},
	}
}

// ValidateGardenletChartRBAC validates the RBAC resources of the Gardenlet chart.
func ValidateGardenletChartRBAC(ctx context.Context, c client.Client, expectedLabels map[string]string, serviceAccountName string) {
	// ClusterRoles
	gardenletClusterRole := getGardenletClusterRole(expectedLabels)
	apiServerSNIClusterRole := getAPIServerSNIClusterRole(expectedLabels)
	managedIstioClusterRole := getManagedIstioClusterRole(expectedLabels)
	expectedClusterRoles := map[types.NamespacedName]*rbacv1.ClusterRole{
		{Name: gardenletClusterRole.Name}:    gardenletClusterRole,
		{Name: apiServerSNIClusterRole.Name}: apiServerSNIClusterRole,
		{Name: managedIstioClusterRole.Name}: managedIstioClusterRole,
	}
	for key, expected := range expectedClusterRoles {
		actual := &rbacv1.ClusterRole{}
		Expect(c.Get(ctx, key, actual)).ToNot(HaveOccurred())

		Expect(actual).To(DeepEqual(expected))
	}

	// ClusterRoleBindings
	gardenletClusterRoleBinding := getGardenletClusterRoleBinding(expectedLabels, serviceAccountName)
	apiServerSNIClusterRoleBinding := getAPIServerSNIClusterRoleBinding(expectedLabels, serviceAccountName)
	managedIstioClusterRoleBinding := getManagedIstioClusterRoleBinding(expectedLabels, serviceAccountName)
	expectedClusterRoleBindings := map[types.NamespacedName]*rbacv1.ClusterRoleBinding{
		{Name: gardenletClusterRoleBinding.Name}:    gardenletClusterRoleBinding,
		{Name: apiServerSNIClusterRoleBinding.Name}: apiServerSNIClusterRoleBinding,
		{Name: managedIstioClusterRoleBinding.Name}: managedIstioClusterRoleBinding,
	}
	for key, expected := range expectedClusterRoleBindings {
		actual := &rbacv1.ClusterRoleBinding{}
		Expect(c.Get(ctx, key, actual)).ToNot(HaveOccurred())

		Expect(actual).To(DeepEqual(expected))
	}

	// Roles
	gardenGardenletRole := getGardenGardenletRole(expectedLabels)
	expectedRoles := map[types.NamespacedName]*rbacv1.Role{
		{Name: gardenGardenletRole.Name, Namespace: gardenGardenletRole.Namespace}: gardenGardenletRole,
	}
	for key, expected := range expectedRoles {
		actual := &rbacv1.Role{}
		Expect(c.Get(ctx, key, actual)).ToNot(HaveOccurred())

		Expect(actual).To(DeepEqual(expected))
	}

	// RoleBindings
	gardenGardenletRoleBinding := getGardenGardenletRoleBinding(expectedLabels, serviceAccountName)
	expectedRoleBindings := map[types.NamespacedName]*rbacv1.RoleBinding{
		{Name: gardenGardenletRoleBinding.Name, Namespace: gardenGardenletRoleBinding.Namespace}: gardenGardenletRoleBinding,
	}
	for key, expected := range expectedRoleBindings {
		actual := &rbacv1.RoleBinding{}
		Expect(c.Get(ctx, key, actual)).ToNot(HaveOccurred())

		Expect(actual).To(DeepEqual(expected))
	}
}

func getGardenletClusterRole(labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{Kind: "ClusterRole", APIVersion: rbacv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gardener.cloud:system:gardenlet",
			Labels:          labels,
			ResourceVersion: "1",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"operator.gardener.cloud"},
				Resources: []string{"gardens"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"endpoints", "persistentvolumes"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumes"},
				Verbs:     []string{"patch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"list", "watch", "delete", "deletecollection"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods/log"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods/exec"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps", "namespaces", "secrets", "serviceaccounts", "services"},
				Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups:     []string{""},
				Resources:     []string{"persistentvolumeclaims"},
				ResourceNames: []string{"vali-vali-0"},
				Verbs:         []string{"delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"get", "list", "create", "patch", "update"},
			},
			{
				APIGroups: []string{"admissionregistration.k8s.io"},
				Resources: []string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"},
				Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"apiextensions.k8s.io"},
				Resources: []string{"customresourcedefinitions"},
				ResourceNames: []string{
					"etcds.druid.gardener.cloud",
					"etcdcopybackupstasks.druid.gardener.cloud",
					"destinationrules.networking.istio.io",
					"envoyfilters.networking.istio.io",
					"gateways.networking.istio.io",
					"serviceentries.networking.istio.io",
					"sidecars.networking.istio.io",
					"virtualservices.networking.istio.io",
					"authorizationpolicies.security.istio.io",
					"peerauthentications.security.istio.io",
					"requestauthentications.security.istio.io",
					"workloadentries.networking.istio.io",
					"workloadgroups.networking.istio.io",
					"telemetries.telemetry.istio.io",
					"wasmplugins.extensions.istio.io",
					"proxyconfigs.networking.istio.io",
					"managedresources.resources.gardener.cloud",
					"clusterfilters.fluentbit.fluent.io",
					"clusterfluentbitconfigs.fluentbit.fluent.io",
					"clusterinputs.fluentbit.fluent.io",
					"clusteroutputs.fluentbit.fluent.io",
					"clusterparsers.fluentbit.fluent.io",
					"clustermultilineparsers.fluentbit.fluent.io",
					"fluentbits.fluentbit.fluent.io",
					"collectors.fluentbit.fluent.io",
					"fluentbitconfigs.fluentbit.fluent.io",
					"filters.fluentbit.fluent.io",
					"outputs.fluentbit.fluent.io",
					"parsers.fluentbit.fluent.io",
					"multilineparsers.fluentbit.fluent.io",
				},
				Verbs: []string{"delete"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "deployments/scale", "statefulsets", "statefulsets/scale", "replicasets"},
				Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"autoscaling"},
				Resources: []string{"horizontalpodautoscalers"},
				Verbs:     []string{"create", "delete", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"autoscaling.k8s.io"},
				Resources: []string{"verticalpodautoscalers"},
				Verbs:     []string{"create", "delete", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"druid.gardener.cloud"},
				Resources: []string{"etcds", "etcdcopybackupstasks"},
				Verbs:     []string{"create", "delete", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"extensions.gardener.cloud"},
				Resources: []string{"backupbuckets", "backupentries", "bastions", "clusters", "containerruntimes", "controlplanes", "dnsrecords", "extensions", "infrastructures", "networks", "operatingsystemconfigs", "workers"},
				Verbs:     []string{"create", "delete", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"extensions.gardener.cloud"},
				Resources: []string{"backupbuckets/status", "backupentries/status", "containerruntimes/status", "controlplanes/status", "dnsrecords/status", "extensions/status", "infrastructures/status", "networks/status", "operatingsystemconfigs/status", "workers/status"},
				Verbs:     []string{"patch", "update"},
			},
			{
				APIGroups: []string{"resources.gardener.cloud"},
				Resources: []string{"managedresources"},
				Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"networkpolicies"},
				Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"extensions", "networking.k8s.io"},
				Resources: []string{"ingresses", "ingressclasses"},
				Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"policy"},
				Resources: []string{"poddisruptionbudgets"},
				Verbs:     []string{"create", "delete", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"rbac.authorization.k8s.io"},
				Resources: []string{"clusterrolebindings", "clusterroles", "rolebindings", "roles"},
				Verbs:     []string{"create", "delete", "deletecollection", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"rbac.authorization.k8s.io"},
				Resources: []string{"clusterroles", "roles"},
				Verbs:     []string{"bind", "escalate"},
			},
			{
				APIGroups: []string{"scheduling.k8s.io"},
				Resources: []string{"priorityclasses"},
				Verbs:     []string{"create", "delete", "get", "list", "watch", "patch", "update"},
			},
			{
				NonResourceURLs: []string{"/healthz", "/version"},
				Verbs:           []string{"get"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"create", "get", "list", "watch"},
			},
			{
				APIGroups:     []string{"coordination.k8s.io"},
				Resources:     []string{"leases"},
				ResourceNames: []string{"gardenlet-leader-election"},
				Verbs:         []string{"update"},
			},
			{
				APIGroups:     []string{"coordination.k8s.io"},
				Resources:     []string{"leases"},
				ResourceNames: []string{"gardener-extension-heartbeat"},
				Verbs:         []string{"get"},
			},
			{
				APIGroups:     []string{"networking.istio.io"},
				Resources:     []string{"virtualservices"},
				ResourceNames: []string{"kube-apiserver"},
				Verbs:         []string{"list"},
			},
			{
				APIGroups: []string{"networking.istio.io"},
				Resources: []string{"destinationrules", "gateways", "virtualservices", "envoyfilters"},
				Verbs:     []string{"delete"},
			},
			{
				APIGroups: []string{"machine.sapcloud.io"},
				Resources: []string{"machineclasses", "machinedeployments", "machinesets", "machines"},
				Verbs:     []string{"list", "watch", "get", "patch", "update", "delete"},
			},
			{
				APIGroups: []string{"monitoring.coreos.com"},
				Resources: []string{"servicemonitors", "scrapeconfigs", "prometheusrules"},
				Verbs:     []string{"list", "watch", "get", "create", "patch", "update", "delete"},
			},
		},
	}
}

func getAPIServerSNIClusterRole(labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{Kind: "ClusterRole", APIVersion: rbacv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gardener.cloud:system:gardenlet:apiserver-sni",
			Labels:          labels,
			ResourceVersion: "1",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"networking.istio.io"},
				Resources: []string{"envoyfilters", "gateways", "virtualservices"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{"networking.istio.io"},
				Resources:     []string{"envoyfilters", "gateways"},
				ResourceNames: []string{"proxy-protocol"},
				Verbs:         []string{"get", "patch", "update"},
			},
			{
				APIGroups:     []string{"networking.istio.io"},
				Resources:     []string{"virtualservices"},
				ResourceNames: []string{"proxy-protocol-blackhole"},
				Verbs:         []string{"get", "patch", "update"},
			},
		},
	}
}

func getManagedIstioClusterRole(labels map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{Kind: "ClusterRole", APIVersion: rbacv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gardener.cloud:system:gardenlet:managed-istio",
			Labels:          labels,
			ResourceVersion: "1",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"networking.istio.io"},
				Resources: []string{"destinationrules", "gateways", "virtualservices", "envoyfilters", "sidecars"},
				Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
			},
			{
				APIGroups: []string{"security.istio.io"},
				Resources: []string{"peerauthentications"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups:     []string{"security.istio.io"},
				Resources:     []string{"peerauthentications"},
				ResourceNames: []string{"default"},
				Verbs:         []string{"get", "patch", "update"},
			},
			{
				APIGroups:     []string{"admissionregistration.k8s.io"},
				Resources:     []string{"validatingwebhookconfigurations"},
				ResourceNames: []string{"istiod"},
				Verbs:         []string{"get", "patch", "update"},
			},
		},
	}
}

func getGardenletClusterRoleBinding(labels map[string]string, serviceAccountName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{Kind: "ClusterRoleBinding", APIVersion: rbacv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gardener.cloud:system:gardenlet",
			Labels:          labels,
			ResourceVersion: "1",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     "gardener.cloud:system:gardenlet",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: v1beta1constants.GardenNamespace,
			},
		},
	}
}

func getAPIServerSNIClusterRoleBinding(labels map[string]string, serviceAccountName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{Kind: "ClusterRoleBinding", APIVersion: rbacv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gardener.cloud:system:gardenlet:apiserver-sni",
			Labels:          labels,
			ResourceVersion: "1",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     "gardener.cloud:system:gardenlet:apiserver-sni",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: v1beta1constants.GardenNamespace,
			},
		},
	}
}

func getManagedIstioClusterRoleBinding(labels map[string]string, serviceAccountName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{Kind: "ClusterRoleBinding", APIVersion: rbacv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gardener.cloud:system:gardenlet:managed-istio",
			Labels:          labels,
			ResourceVersion: "1",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "ClusterRole",
			Name:     "gardener.cloud:system:gardenlet:managed-istio",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: v1beta1constants.GardenNamespace,
			},
		},
	}
}

func getGardenGardenletRole(labels map[string]string) *rbacv1.Role {
	return &rbacv1.Role{
		TypeMeta: metav1.TypeMeta{Kind: "Role", APIVersion: rbacv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gardener.cloud:system:gardenlet",
			Namespace:       "garden",
			Labels:          labels,
			ResourceVersion: "1",
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps"},
				Resources: []string{"daemonsets"},
				Verbs:     []string{"create"},
			},
		},
	}
}

func getGardenGardenletRoleBinding(labels map[string]string, serviceAccountName string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{Kind: "RoleBinding", APIVersion: rbacv1.SchemeGroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "gardener.cloud:system:gardenlet",
			Namespace:       "garden",
			Labels:          labels,
			ResourceVersion: "1",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.SchemeGroupVersion.Group,
			Kind:     "Role",
			Name:     "gardener.cloud:system:gardenlet",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: v1beta1constants.GardenNamespace,
			},
		},
	}
}

// ValidateGardenletChartServiceAccount validates the Service Account of the Gardenlet chart.
func ValidateGardenletChartServiceAccount(ctx context.Context, c client.Client, hasSeedClientConnectionKubeconfig bool, expectedLabels map[string]string, serviceAccountName string) {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: v1beta1constants.GardenNamespace,
		},
	}

	if hasSeedClientConnectionKubeconfig {
		err := c.Get(
			ctx,
			client.ObjectKey{Namespace: serviceAccount.Namespace, Name: serviceAccount.Name},
			serviceAccount,
		)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		return
	}

	expectedServiceAccount := *serviceAccount
	expectedServiceAccount.Labels = expectedLabels

	Expect(c.Get(
		ctx,
		client.ObjectKey{Namespace: serviceAccount.Namespace, Name: serviceAccount.Name},
		serviceAccount,
	)).ToNot(HaveOccurred())
	Expect(serviceAccount.Labels).To(DeepEqual(expectedServiceAccount.Labels))
}

// ValidateGardenletChartPodDisruptionBudget validates the PodDisruptionBudget of the Gardenlet chart.
func ValidateGardenletChartPodDisruptionBudget(ctx context.Context, c client.Client, expectedLabels map[string]string, replicaCount *int32) {
	maxUnavailable := intstr.FromInt32(1)

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet",
			Namespace: v1beta1constants.GardenNamespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			MaxUnavailable:             &maxUnavailable,
			Selector:                   &metav1.LabelSelector{},
			UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
		},
	}

	if ptr.Deref(replicaCount, 2) < 2 {
		Expect(c.Get(ctx, client.ObjectKeyFromObject(pdb), pdb)).To(BeNotFoundError())
		return
	}

	expectedPodDisruptionBudget := *pdb
	expectedPodDisruptionBudget.Labels = expectedLabels
	expectedPodDisruptionBudget.Spec.Selector.MatchLabels = expectedLabels

	Expect(c.Get(ctx, client.ObjectKeyFromObject(pdb), pdb)).To(Succeed())
	Expect(pdb.Labels).To(DeepEqual(expectedPodDisruptionBudget.Labels))
	Expect(pdb.Spec).To(DeepEqual(expectedPodDisruptionBudget.Spec))
}

// ComputeExpectedGardenletConfiguration computes the expected Gardenlet configuration based
// on input parameters.
func ComputeExpectedGardenletConfiguration(
	hasGardenClientConnectionKubeconfig, hasSeedClientConnectionKubeconfig bool,
	bootstrapKubeconfig *corev1.SecretReference,
	kubeconfigSecret *corev1.SecretReference,
	seedConfig *gardenletconfigv1alpha1.SeedConfig,
	featureGates map[string]bool,
) gardenletconfigv1alpha1.GardenletConfiguration {
	var (
		zero   = 0
		one    = 1
		five   = 5
		twenty = 20

		logLevelInfo        = "info"
		logFormatJson       = "json"
		lockObjectName      = "gardenlet-leader-election"
		lockObjectNamespace = "garden"
	)

	config := gardenletconfigv1alpha1.GardenletConfiguration{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GardenletConfiguration",
			APIVersion: "gardenlet.config.gardener.cloud/v1alpha1",
		},
		GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
			ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   100,
				Burst: 130,
			},
			KubeconfigValidity: &gardenletconfigv1alpha1.KubeconfigValidity{
				AutoRotationJitterPercentageMin: ptr.To[int32](70),
				AutoRotationJitterPercentageMax: ptr.To[int32](90),
			},
		},
		SeedClientConnection: &gardenletconfigv1alpha1.SeedClientConnection{
			ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   100,
				Burst: 130,
			},
		},
		ShootClientConnection: &gardenletconfigv1alpha1.ShootClientConnection{
			ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
				QPS:   25,
				Burst: 50,
			},
		},
		Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
			BackupBucket: &gardenletconfigv1alpha1.BackupBucketControllerConfiguration{
				ConcurrentSyncs: &twenty,
			},
			BackupEntry: &gardenletconfigv1alpha1.BackupEntryControllerConfiguration{
				ConcurrentSyncs:          &twenty,
				DeletionGracePeriodHours: &zero,
			},
			Bastion: &gardenletconfigv1alpha1.BastionControllerConfiguration{
				ConcurrentSyncs: &twenty,
			},
			Gardenlet: &gardenletconfigv1alpha1.GardenletObjectControllerConfiguration{
				SyncPeriod: &metav1.Duration{
					Duration: 1 * time.Hour,
				},
			},
			Seed: &gardenletconfigv1alpha1.SeedControllerConfiguration{
				SyncPeriod: &metav1.Duration{
					Duration: 1 * time.Hour,
				},
				LeaseResyncSeconds:       ptr.To[int32](2),
				LeaseResyncMissThreshold: ptr.To[int32](10),
			},
			Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
				ReconcileInMaintenanceOnly: ptr.To(false),
				RespectSyncPeriodOverwrite: ptr.To(false),
				ConcurrentSyncs:            &twenty,
				SyncPeriod: &metav1.Duration{
					Duration: time.Hour,
				},
				RetryDuration: &metav1.Duration{
					Duration: 12 * time.Hour,
				},
				DNSEntryTTLSeconds: ptr.To[int64](120),
			},
			ManagedSeed: &gardenletconfigv1alpha1.ManagedSeedControllerConfiguration{
				ConcurrentSyncs: &five,
				JitterUpdates:   ptr.To(false),
				SyncPeriod: &metav1.Duration{
					Duration: 1 * time.Hour,
				},
				WaitSyncPeriod: &metav1.Duration{
					Duration: 15 * time.Second,
				},
				SyncJitterPeriod: &metav1.Duration{
					Duration: 300000000000,
				},
			},
			ShootCare: &gardenletconfigv1alpha1.ShootCareControllerConfiguration{
				ConcurrentSyncs: &five,
				SyncPeriod: &metav1.Duration{
					Duration: 30 * time.Second,
				},
				StaleExtensionHealthChecks: &gardenletconfigv1alpha1.StaleExtensionHealthChecks{
					Enabled:   true,
					Threshold: &metav1.Duration{Duration: 300000000000},
				},
				ManagedResourceProgressingThreshold: &metav1.Duration{Duration: time.Hour},
				ConditionThresholds: []gardenletconfigv1alpha1.ConditionThreshold{
					{
						Type: string(gardencorev1beta1.ShootAPIServerAvailable),
						Duration: metav1.Duration{
							Duration: 1 * time.Minute,
						},
					},
					{
						Type: string(gardencorev1beta1.ShootControlPlaneHealthy),
						Duration: metav1.Duration{
							Duration: 1 * time.Minute,
						},
					},
					{
						Type: string(gardencorev1beta1.ShootObservabilityComponentsHealthy),
						Duration: metav1.Duration{
							Duration: 1 * time.Minute,
						},
					},
					{
						Type: string(gardencorev1beta1.ShootSystemComponentsHealthy),
						Duration: metav1.Duration{
							Duration: 1 * time.Minute,
						},
					},
					{
						Type: string(gardencorev1beta1.ShootEveryNodeReady),
						Duration: metav1.Duration{
							Duration: 5 * time.Minute,
						},
					},
				},
				WebhookRemediatorEnabled: ptr.To(false),
			},
			SeedCare: &gardenletconfigv1alpha1.SeedCareControllerConfiguration{
				SyncPeriod: &metav1.Duration{
					Duration: 30 * time.Second,
				},
				ConditionThresholds: []gardenletconfigv1alpha1.ConditionThreshold{
					{
						Type: string(gardencorev1beta1.SeedSystemComponentsHealthy),
						Duration: metav1.Duration{
							Duration: 1 * time.Minute,
						},
					},
				},
			},
			ShootState: &gardenletconfigv1alpha1.ShootStateControllerConfiguration{
				ConcurrentSyncs: &five,
				SyncPeriod:      &metav1.Duration{Duration: 6 * time.Hour},
			},
			TokenRequestorServiceAccount: &gardenletconfigv1alpha1.TokenRequestorServiceAccountControllerConfiguration{
				ConcurrentSyncs: &five,
			},
			TokenRequestorWorkloadIdentity: &gardenletconfigv1alpha1.TokenRequestorWorkloadIdentityControllerConfiguration{
				ConcurrentSyncs: &five,
			},
			VPAEvictionRequirements: &gardenletconfigv1alpha1.VPAEvictionRequirementsControllerConfiguration{
				ConcurrentSyncs: &five,
			},
			ControllerInstallation: &gardenletconfigv1alpha1.ControllerInstallationControllerConfiguration{
				ConcurrentSyncs: &twenty,
			},
			ControllerInstallationCare: &gardenletconfigv1alpha1.ControllerInstallationCareControllerConfiguration{
				ConcurrentSyncs: &twenty,
				SyncPeriod:      &metav1.Duration{Duration: 30 * time.Second},
			},
			ControllerInstallationRequired: &gardenletconfigv1alpha1.ControllerInstallationRequiredControllerConfiguration{
				ConcurrentSyncs: &one,
			},
			NetworkPolicy: &gardenletconfigv1alpha1.NetworkPolicyControllerConfiguration{
				ConcurrentSyncs: &five,
			},
		},
		LeaderElection: &componentbaseconfigv1alpha1.LeaderElectionConfiguration{
			LeaderElect:       ptr.To(true),
			LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
			RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
			RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceName:      lockObjectName,
			ResourceNamespace: lockObjectNamespace,
		},
		LogLevel:  logLevelInfo,
		LogFormat: logFormatJson,
		Logging: &gardenletconfigv1alpha1.Logging{
			Enabled: ptr.To(false),
			Vali: &gardenletconfigv1alpha1.Vali{
				Enabled: ptr.To(false),
				Garden: &gardenletconfigv1alpha1.GardenVali{
					Storage: &gardenletconfigv1alpha1.DefaultCentralValiStorage,
				},
			},
			ShootEventLogging: &gardenletconfigv1alpha1.ShootEventLogging{
				Enabled: ptr.To(false),
			},
		},
		Server: gardenletconfigv1alpha1.ServerConfiguration{
			HealthProbes: &gardenletconfigv1alpha1.Server{
				Port: 2728,
			},
			Metrics: &gardenletconfigv1alpha1.Server{
				Port: 2729,
			},
		},
		Debugging: &componentbaseconfigv1alpha1.DebuggingConfiguration{
			EnableProfiling:           ptr.To(false),
			EnableContentionProfiling: ptr.To(false),
		},
		FeatureGates: featureGates,
		Resources: &gardenletconfigv1alpha1.ResourcesConfiguration{
			Capacity: corev1.ResourceList{
				"shoots": resource.MustParse("250"),
			},
		},
		SNI: &gardenletconfigv1alpha1.SNI{Ingress: &gardenletconfigv1alpha1.SNIIngress{
			ServiceName: ptr.To(v1beta1constants.DefaultSNIIngressServiceName),
			Namespace:   ptr.To(v1beta1constants.DefaultSNIIngressNamespace),
			Labels:      map[string]string{"app": "istio-ingressgateway", "istio": "ingressgateway"},
		}},
		Monitoring: &gardenletconfigv1alpha1.MonitoringConfig{
			Shoot: &gardenletconfigv1alpha1.ShootMonitoringConfig{
				Enabled: ptr.To(true),
			},
		},
		ETCDConfig: &gardenletconfigv1alpha1.ETCDConfig{
			BackupCompactionController: &gardenletconfigv1alpha1.BackupCompactionController{
				EnableBackupCompaction:    ptr.To(false),
				EventsThreshold:           ptr.To[int64](1000000),
				MetricsScrapeWaitDuration: &metav1.Duration{Duration: 60 * time.Second},
				Workers:                   ptr.To[int64](3),
			},
			CustodianController: &gardenletconfigv1alpha1.CustodianController{
				Workers: ptr.To[int64](10),
			},
			ETCDController: &gardenletconfigv1alpha1.ETCDController{
				Workers: ptr.To[int64](50),
			},
		},
		NodeToleration: &gardenletconfigv1alpha1.NodeToleration{
			DefaultNotReadyTolerationSeconds:    ptr.To[int64](60),
			DefaultUnreachableTolerationSeconds: ptr.To[int64](60),
		},
	}

	if hasGardenClientConnectionKubeconfig {
		config.GardenClientConnection.Kubeconfig = "/etc/gardenlet/kubeconfig-garden/kubeconfig"
	}

	if hasSeedClientConnectionKubeconfig {
		config.SeedClientConnection.Kubeconfig = "/etc/gardenlet/kubeconfig-seed/kubeconfig"
	}

	if bootstrapKubeconfig != nil {
		config.GardenClientConnection.BootstrapKubeconfig = bootstrapKubeconfig
	}
	config.GardenClientConnection.KubeconfigSecret = kubeconfigSecret

	if seedConfig != nil {
		config.SeedConfig = seedConfig
	}

	return config
}

// VerifyGardenletComponentConfigConfigMap verifies that the actual Gardenlet component config config map equals the expected config map.
func VerifyGardenletComponentConfigConfigMap(
	ctx context.Context,
	c client.Client,
	universalDecoder runtime.Decoder,
	expectedGardenletConfig gardenletconfigv1alpha1.GardenletConfiguration,
	expectedLabels map[string]string,
	uniqueName string,
) {
	componentConfigCm := getEmptyGardenletConfigMap()
	expectedComponentConfigCm := getEmptyGardenletConfigMap()
	expectedComponentConfigCm.Labels = expectedLabels

	if err := c.Get(ctx, client.ObjectKey{Namespace: componentConfigCm.Namespace, Name: uniqueName}, componentConfigCm); err != nil {
		if !apierrors.IsNotFound(err) {
			ginkgo.Fail(err.Error())
		}
		list := &corev1.ConfigMapList{}
		Expect(c.List(ctx, list)).ToNot(HaveOccurred())
		cmNames := ""
		for _, cm := range list.Items {
			cmNames += " " + cm.Name
		}
		ginkgo.Fail("Could not find unique gardenlet configmap " + uniqueName + ", possibly the unique name has changed. Found:" + cmNames)
	}

	Expect(componentConfigCm.Labels).To(DeepEqual(expectedComponentConfigCm.Labels))

	// unmarshal Gardenlet Configuration from deployed Config Map
	componentConfigYaml := componentConfigCm.Data["config.yaml"]
	Expect(componentConfigYaml).ToNot(BeEmpty())
	gardenletConfig := &gardenletconfigv1alpha1.GardenletConfiguration{}
	_, _, err := universalDecoder.Decode([]byte(componentConfigYaml), nil, gardenletConfig)
	Expect(err).ToNot(HaveOccurred())
	Expect(*gardenletConfig).To(DeepEqual(expectedGardenletConfig))
}

func getEmptyGardenletConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet-configmap",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}

// ComputeExpectedGardenletDeploymentSpec computes the expected Gardenlet deployment spec based on input parameters
// needs to equal exactly what is deployed via the helm chart (including defaults set in the helm chart)
// as a consequence, if non-optional changes to the helm chart are made, these tests will fail by design
func ComputeExpectedGardenletDeploymentSpec(
	deploymentConfiguration *seedmanagement.GardenletDeployment,
	image seedmanagement.Image,
	gardenClientConnectionKubeconfig, seedClientConnectionKubeconfig *string,
	expectedLabels map[string]string,
	imageVectorOverwrite, componentImageVectorOverwrites *string,
	uniqueName map[string]string,
	seedConfig *gardenletconfigv1alpha1.SeedConfig,
) (
	appsv1.DeploymentSpec,
	error,
) {
	if image.Repository == nil || image.Tag == nil {
		return appsv1.DeploymentSpec{}, errors.New("the image repository and tag must be provided")
	}

	deployment := appsv1.DeploymentSpec{
		RevisionHistoryLimit: ptr.To[int32](2),
		Replicas:             ptr.To[int32](2),
		Selector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app":  "gardener",
				"role": "gardenlet",
			},
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: expectedLabels,
			},
			Spec: corev1.PodSpec{
				PriorityClassName:  v1beta1constants.PriorityClassNameSeedSystemCritical,
				ServiceAccountName: "gardenlet",
				SecurityContext: &corev1.PodSecurityContext{
					RunAsNonRoot: ptr.To(true),
					RunAsUser:    ptr.To[int64](65532),
					RunAsGroup:   ptr.To[int64](65532),
					FSGroup:      ptr.To[int64](65532),
					SeccompProfile: &corev1.SeccompProfile{
						Type: corev1.SeccompProfileTypeRuntimeDefault,
					},
				},
				Containers: []corev1.Container{
					{
						Name:            "gardenlet",
						Image:           fmt.Sprintf("%s:%s", *image.Repository, *image.Tag),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Args: []string{
							"--config=/etc/gardenlet/config/config.yaml",
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/healthz",
									Port:   intstr.IntOrString{IntVal: 2728},
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 15,
							TimeoutSeconds:      5,
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readyz",
									Port:   intstr.IntOrString{IntVal: 2728},
									Scheme: corev1.URISchemeHTTP,
								},
							},
							InitialDelaySeconds: 10,
							TimeoutSeconds:      5,
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
						},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: ptr.To(false),
						},
						TerminationMessagePath:   "/dev/termination-log",
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "kube-api-access-gardener",
							MountPath: "/var/run/secrets/kubernetes.io/serviceaccount",
							ReadOnly:  true,
						}},
					},
				},
				Tolerations: []corev1.Toleration{
					{
						Key:               "node.kubernetes.io/not-ready",
						Operator:          "Exists",
						TolerationSeconds: ptr.To[int64](60),
						Effect:            "NoExecute",
					},
					{
						Key:               "node.kubernetes.io/unreachable",
						Operator:          "Exists",
						TolerationSeconds: ptr.To[int64](60),
						Effect:            "NoExecute",
					},
				},
				Volumes: []corev1.Volume{{
					Name: "kube-api-access-gardener",
					VolumeSource: corev1.VolumeSource{
						Projected: &corev1.ProjectedVolumeSource{
							DefaultMode: ptr.To[int32](420),
							Sources: []corev1.VolumeProjection{
								{ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
									Path:              "token",
									ExpirationSeconds: ptr.To[int64](43200),
								}},
								{ConfigMap: &corev1.ConfigMapProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kube-root-ca.crt",
									},
									Items: []corev1.KeyToPath{{
										Key:  "ca.crt",
										Path: "ca.crt",
									}},
								}},
								{DownwardAPI: &corev1.DownwardAPIProjection{
									Items: []corev1.DownwardAPIVolumeFile{{
										Path: "namespace",
										FieldRef: &corev1.ObjectFieldSelector{
											APIVersion: "v1",
											FieldPath:  "metadata.namespace",
										},
									}},
								}},
							},
						},
					},
				}},
			},
		},
	}

	if deploymentConfiguration != nil {
		if deploymentConfiguration.RevisionHistoryLimit != nil {
			deployment.RevisionHistoryLimit = deploymentConfiguration.RevisionHistoryLimit
		}

		if deploymentConfiguration.ServiceAccountName != nil {
			deployment.Template.Spec.ServiceAccountName = *deploymentConfiguration.ServiceAccountName
		}

		if deploymentConfiguration.ReplicaCount != nil {
			deployment.Replicas = deploymentConfiguration.ReplicaCount
		}

		if replicas := ptr.Deref(deployment.Replicas, 2); replicas > 1 {
			numberOfZones := 1
			if seedConfig != nil {
				numberOfZones = len(seedConfig.Spec.Provider.Zones)
			}

			deployment.Template.Spec.TopologySpreadConstraints = kubernetesutils.GetTopologySpreadConstraints(
				replicas,
				replicas,
				metav1.LabelSelector{MatchLabels: map[string]string{"app": "gardener", "role": "gardenlet"}},
				int32(numberOfZones), // #nosec G115 -- `len(seedConfig.Spec.Provider.Zones)` cannot be higher than max int32. Zones come from shoot spec and there is a validation that there cannot be more zones than worker.Maximum which is int32.
				nil,
				false,
			)
		}

		if deploymentConfiguration.Env != nil {
			deployment.Template.Spec.Containers[0].Env = deploymentConfiguration.Env
		}

		if deploymentConfiguration.PodLabels != nil {
			deployment.Template.ObjectMeta.Labels = utils.MergeStringMaps(deployment.Template.ObjectMeta.Labels, deploymentConfiguration.PodLabels)
		}

		if deploymentConfiguration.PodAnnotations != nil {
			deployment.Template.ObjectMeta.Annotations = utils.MergeStringMaps(deployment.Template.ObjectMeta.Annotations, deploymentConfiguration.PodAnnotations)
		}

		if deploymentConfiguration.Resources != nil {
			if value, ok := deploymentConfiguration.Resources.Requests[corev1.ResourceCPU]; ok {
				deployment.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = value
			}

			if value, ok := deploymentConfiguration.Resources.Requests[corev1.ResourceMemory]; ok {
				deployment.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = value
			}

			if value, ok := deploymentConfiguration.Resources.Limits[corev1.ResourceCPU]; ok {
				if deployment.Template.Spec.Containers[0].Resources.Limits == nil {
					deployment.Template.Spec.Containers[0].Resources.Limits = map[corev1.ResourceName]resource.Quantity{}
				}
				deployment.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU] = value
			}
			if value, ok := deploymentConfiguration.Resources.Limits[corev1.ResourceMemory]; ok {
				if deployment.Template.Spec.Containers[0].Resources.Limits == nil {
					deployment.Template.Spec.Containers[0].Resources.Limits = map[corev1.ResourceName]resource.Quantity{}
				}
				deployment.Template.Spec.Containers[0].Resources.Limits[corev1.ResourceMemory] = value
			}
		}
	}

	if imageVectorOverwrite != nil {
		deployment.Template.Spec.Containers[0].Env = append(deployment.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "IMAGEVECTOR_OVERWRITE",
			Value: "/imagevector_overwrite/images_overwrite.yaml",
		})
		deployment.Template.Spec.Containers[0].VolumeMounts = append(deployment.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "gardenlet-imagevector-overwrite",
			ReadOnly:  true,
			MountPath: "/imagevector_overwrite",
		})
		deployment.Template.Spec.Volumes = append(deployment.Template.Spec.Volumes, corev1.Volume{
			Name: "gardenlet-imagevector-overwrite",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: uniqueName["gardenlet-imagevector-overwrite"],
					},
				},
			},
		})
	}

	if componentImageVectorOverwrites != nil {
		deployment.Template.Spec.Containers[0].Env = append(deployment.Template.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  "IMAGEVECTOR_OVERWRITE_COMPONENTS",
			Value: "/imagevector_overwrite_components/components.yaml",
		})
		deployment.Template.Spec.Containers[0].VolumeMounts = append(deployment.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "gardenlet-imagevector-overwrite-components",
			ReadOnly:  true,
			MountPath: "/imagevector_overwrite_components",
		})
		deployment.Template.Spec.Volumes = append(deployment.Template.Spec.Volumes, corev1.Volume{
			Name: "gardenlet-imagevector-overwrite-components",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: uniqueName["gardenlet-imagevector-overwrite-components"],
					},
				},
			},
		})
	}

	if gardenClientConnectionKubeconfig != nil {
		deployment.Template.Spec.Containers[0].VolumeMounts = append(deployment.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "gardenlet-kubeconfig-garden",
			MountPath: "/etc/gardenlet/kubeconfig-garden",
			ReadOnly:  true,
		})
		deployment.Template.Spec.Volumes = append(deployment.Template.Spec.Volumes, corev1.Volume{
			Name: "gardenlet-kubeconfig-garden",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: uniqueName["gardenlet-kubeconfig-garden"],
				},
			},
		})
	}

	if seedClientConnectionKubeconfig != nil {
		deployment.Template.Spec.Containers[0].VolumeMounts = append(deployment.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "gardenlet-kubeconfig-seed",
			MountPath: "/etc/gardenlet/kubeconfig-seed",
			ReadOnly:  true,
		})
		deployment.Template.Spec.Volumes = append(deployment.Template.Spec.Volumes, corev1.Volume{
			Name: "gardenlet-kubeconfig-seed",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: uniqueName["gardenlet-kubeconfig-seed"],
				},
			},
		})
		deployment.Template.Spec.ServiceAccountName = ""
		deployment.Template.Spec.AutomountServiceAccountToken = ptr.To(false)
	}

	deployment.Template.Spec.Containers[0].VolumeMounts = append(deployment.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
		Name:      "gardenlet-config",
		MountPath: "/etc/gardenlet/config",
	})

	deployment.Template.Spec.Volumes = append(deployment.Template.Spec.Volumes, corev1.Volume{
		Name: "gardenlet-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: uniqueName["gardenlet-configmap"],
				},
			},
		},
	})

	if deploymentConfiguration != nil && deploymentConfiguration.AdditionalVolumeMounts != nil {
		deployment.Template.Spec.Containers[0].VolumeMounts = append(deployment.Template.Spec.Containers[0].VolumeMounts, deploymentConfiguration.AdditionalVolumeMounts...)
	}

	if deploymentConfiguration != nil && deploymentConfiguration.AdditionalVolumes != nil {
		deployment.Template.Spec.Volumes = append(deployment.Template.Spec.Volumes, deploymentConfiguration.AdditionalVolumes...)
	}

	return deployment, nil
}

// VerifyGardenletDeployment verifies that the actual Gardenlet deployment equals the expected deployment
func VerifyGardenletDeployment(ctx context.Context,
	c client.Client,
	expectedDeploymentSpec appsv1.DeploymentSpec,
	deploymentConfiguration *seedmanagement.GardenletDeployment,
	hasGardenClientConnectionKubeconfig,
	hasSeedClientConnectionKubeconfig,
	usesTLSBootstrapping bool,
	expectedLabels map[string]string,
	imageVectorOverwrite,
	componentImageVectorOverwrites *string,
	uniqueName map[string]string) {
	deployment := getEmptyGardenletDeployment()
	expectedDeployment := getEmptyGardenletDeployment()
	expectedDeployment.Labels = utils.MergeStringMaps(expectedLabels, map[string]string{
		resourcesv1alpha1.HighAvailabilityConfigSkip: "true",
	})

	Expect(c.Get(
		ctx,
		client.ObjectKey{Namespace: deployment.Namespace, Name: deployment.Name},
		deployment,
	)).ToNot(HaveOccurred())

	Expect(deployment.ObjectMeta.Labels).To(DeepEqual(expectedDeployment.ObjectMeta.Labels))

	assertResourceReferenceExists(uniqueName["gardenlet-configmap"], "configmap-", deployment.Spec.Template.Annotations)

	if imageVectorOverwrite != nil {
		assertResourceReferenceExists(uniqueName["gardenlet-imagevector-overwrite"], "configmap-", deployment.Spec.Template.Annotations)
	}

	if componentImageVectorOverwrites != nil {
		assertResourceReferenceExists(uniqueName["gardenlet-imagevector-overwrite-components"], "configmap-", deployment.Spec.Template.Annotations)
	}

	if hasGardenClientConnectionKubeconfig {
		assertResourceReferenceExists(uniqueName["gardenlet-kubeconfig-garden"], "secret-", deployment.Spec.Template.Annotations)
	}

	if hasSeedClientConnectionKubeconfig {
		assertResourceReferenceExists(uniqueName["gardenlet-kubeconfig-seed"], "secret-", deployment.Spec.Template.Annotations)
	}

	if usesTLSBootstrapping {
		Expect(deployment.Spec.Template.Annotations["checksum/secret-gardenlet-kubeconfig-garden-bootstrap"]).ToNot(BeEmpty())
	}

	if deploymentConfiguration != nil && deploymentConfiguration.PodAnnotations != nil {
		for key, value := range deploymentConfiguration.PodAnnotations {
			Expect(deployment.Spec.Template.Annotations[key]).To(Equal(value))
		}
	}

	// clean annotations with hashes
	deployment.Spec.Template.Annotations = nil
	expectedDeploymentSpec.Template.Annotations = nil
	Expect(deployment.Spec).To(DeepEqual(expectedDeploymentSpec))
}

func assertResourceReferenceExists(secretName, prefix string, annotations map[string]string) {
	suffix := utils.ComputeSHA256Hex([]byte(secretName))[:8]
	Expect(annotations["reference.resources.gardener.cloud/"+prefix+suffix]).To(Equal(secretName))
}

func getEmptyGardenletDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "gardenlet",
			Namespace: v1beta1constants.GardenNamespace,
		},
	}
}
