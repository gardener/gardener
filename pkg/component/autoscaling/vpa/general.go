// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package vpa

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component"
	vpaconstants "github.com/gardener/gardener/pkg/component/autoscaling/vpa/constants"
)

const (
	metricsPortName = "metrics"
	serverPortName  = "server"
)

func newDefaultLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   "/health-check",
				Port:   intstr.FromString(metricsPortName),
				Scheme: corev1.URISchemeHTTP,
			},
		},
		// Typically, the short-term impact of losing VPA is low
		// So, we can afford relaxed liveness timing and avoid unnecessary restarts aggravating high load situations
		InitialDelaySeconds: 120,
		PeriodSeconds:       60,
		TimeoutSeconds:      30,
		SuccessThreshold:    1,
		FailureThreshold:    3,
	}
}

func (v *vpa) generalResourceConfigs() component.ResourceConfigs {
	var (
		clusterRoleActor               = v.emptyClusterRole("actor")
		clusterRoleBindingActor        = v.emptyClusterRoleBinding("actor")
		clusterRoleTargetReader        = v.emptyClusterRole("target-reader")
		clusterRoleBindingTargetReader = v.emptyClusterRoleBinding("target-reader")
		mutatingWebhookConfiguration   = v.emptyMutatingWebhookConfiguration()
	)

	return component.ResourceConfigs{
		{Obj: clusterRoleActor, Class: component.Application, MutateFn: func() { v.reconcileGeneralClusterRoleActor(clusterRoleActor) }},
		{Obj: clusterRoleBindingActor, Class: component.Application, MutateFn: func() { v.reconcileGeneralClusterRoleBindingActor(clusterRoleBindingActor, clusterRoleActor) }},
		{Obj: clusterRoleTargetReader, Class: component.Application, MutateFn: func() { v.reconcileGeneralClusterRoleTargetReader(clusterRoleTargetReader) }},
		{Obj: clusterRoleBindingTargetReader, Class: component.Application, MutateFn: func() {
			v.reconcileGeneralClusterRoleBindingTargetReader(clusterRoleBindingTargetReader, clusterRoleTargetReader)
		}},
		{Obj: mutatingWebhookConfiguration, Class: component.Application, MutateFn: func() { v.reconcileGeneralMutatingWebhookConfiguration(mutatingWebhookConfiguration) }},
	}
}

func (v *vpa) reconcileGeneralClusterRoleActor(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"pods", "nodes", "limitranges"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"", "events.k8s.io"},
			Resources: []string{"events"},
			Verbs:     []string{"create", "get", "list", "watch", "patch", "update"},
		},
		{
			APIGroups: []string{"autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"coordination.k8s.io"},
			Resources: []string{"leases"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
}

func (v *vpa) reconcileGeneralClusterRoleBindingActor(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole) {
	clusterRoleBinding.Labels = getRoleLabel()
	clusterRoleBinding.Annotations = map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"}
	clusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	clusterRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      recommender,
			Namespace: v.namespaceForApplicationClassResource(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      updater,
			Namespace: v.namespaceForApplicationClassResource(),
		},
	}
}

func (v *vpa) reconcileGeneralClusterRoleTargetReader(clusterRole *rbacv1.ClusterRole) {
	clusterRole.Labels = getRoleLabel()
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"*"},
			Resources: []string{"*/scale"},
			Verbs:     []string{"get", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"replicationcontrollers"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs", "cronjobs"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"druid.gardener.cloud"},
			Resources: []string{"etcds", "etcds/scale"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
}

func (v *vpa) reconcileGeneralClusterRoleBindingTargetReader(clusterRoleBinding *rbacv1.ClusterRoleBinding, clusterRole *rbacv1.ClusterRole) {
	clusterRoleBinding.Labels = getRoleLabel()
	clusterRoleBinding.Annotations = map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"}
	clusterRoleBinding.RoleRef = rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     clusterRole.Name,
	}
	clusterRoleBinding.Subjects = []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      admissionController,
			Namespace: v.namespaceForApplicationClassResource(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      recommender,
			Namespace: v.namespaceForApplicationClassResource(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      updater,
			Namespace: v.namespaceForApplicationClassResource(),
		},
	}
}

func (v *vpa) reconcileGeneralMutatingWebhookConfiguration(mutatingWebhookConfiguration *admissionregistrationv1.MutatingWebhookConfiguration) {
	var (
		failurePolicy      = admissionregistrationv1.Ignore
		matchPolicy        = admissionregistrationv1.Exact
		reinvocationPolicy = admissionregistrationv1.IfNeededReinvocationPolicy
		sideEffects        = admissionregistrationv1.SideEffectClassNone
		scope              = admissionregistrationv1.AllScopes

		clientConfig = admissionregistrationv1.WebhookClientConfig{
			CABundle: v.caBundle,
		}
	)

	switch v.values.ClusterType {
	case component.ClusterTypeSeed:
		clientConfig.Service = &admissionregistrationv1.ServiceReference{
			Name:      vpaconstants.AdmissionControllerServiceName,
			Namespace: v.namespace,
			Port:      ptr.To(admissionControllerServicePort),
		}
	case component.ClusterTypeShoot:
		// the port is only respected if register-by-url is true, that's why it's in this if-block
		// if it's false it will not set the port during registration, i.e., it will be defaulted to 443,
		// so the servicePort has to be 443 in this case
		// see https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/pkg/admission-controller/config.go#L70-L74
		clientConfig.URL = ptr.To(fmt.Sprintf("https://%s.%s:%d", vpaconstants.AdmissionControllerServiceName, v.namespace, admissionControllerServicePort))
	}

	metav1.SetMetaDataLabel(&mutatingWebhookConfiguration.ObjectMeta, v1beta1constants.LabelExcludeWebhookFromRemediation, "true")
	metav1.SetMetaDataAnnotation(&mutatingWebhookConfiguration.ObjectMeta, v1beta1constants.GardenerDescription,
		"The order in which MutatingWebhooks are called is determined alphabetically. This webhook's name "+
			"intentionally starts with 'zzz', such that it is called after all other webhooks which inject "+
			"containers. All containers injected by webhooks that are called _after_ the vpa webhook will not be "+
			"under control of vpa.")
	mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
		Name:                    "vpa.k8s.io",
		AdmissionReviewVersions: []string{"v1"},
		ClientConfig:            clientConfig,
		FailurePolicy:           &failurePolicy,
		MatchPolicy:             &matchPolicy,
		ReinvocationPolicy:      &reinvocationPolicy,
		SideEffects:             &sideEffects,
		TimeoutSeconds:          ptr.To[int32](10),
		Rules: []admissionregistrationv1.RuleWithOperations{
			{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
					Scope:       &scope,
				},
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
			},
			{
				Rule: admissionregistrationv1.Rule{
					APIGroups:   []string{"autoscaling.k8s.io"},
					APIVersions: []string{"*"},
					Resources:   []string{"verticalpodautoscalers"},
					Scope:       &scope,
				},
				Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
			},
		},
	}}
}

func (v *vpa) reconcilePodDisruptionBudget(pdb *policyv1.PodDisruptionBudget, deployment *appsv1.Deployment) {
	pdb.Labels = getRoleLabel()
	pdb.Spec = policyv1.PodDisruptionBudgetSpec{
		MaxUnavailable:             ptr.To(intstr.FromInt32(1)),
		Selector:                   deployment.Spec.Selector,
		UnhealthyPodEvictionPolicy: ptr.To(policyv1.AlwaysAllow),
	}
}
