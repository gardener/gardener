// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package vpa

import (
	"fmt"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func (v *vpa) generalResourceConfigs() component.ResourceConfigs {
	var (
		clusterRoleActor               = v.emptyClusterRole("actor")
		clusterRoleBindingActor        = v.emptyClusterRoleBinding("actor")
		clusterRoleTargetReader        = v.emptyClusterRole("target-reader")
		clusterRoleBindingTargetReader = v.emptyClusterRoleBinding("target-reader")
		mutatingWebhookConfguration    = v.emptyMutatingWebhookConfiguration()
	)

	return component.ResourceConfigs{
		{Obj: clusterRoleActor, Class: component.Application, MutateFn: func() { v.reconcileGeneralClusterRoleActor(clusterRoleActor) }},
		{Obj: clusterRoleBindingActor, Class: component.Application, MutateFn: func() { v.reconcileGeneralClusterRoleBindingActor(clusterRoleBindingActor, clusterRoleActor) }},
		{Obj: clusterRoleTargetReader, Class: component.Application, MutateFn: func() { v.reconcileGeneralClusterRoleTargetReader(clusterRoleTargetReader) }},
		{Obj: clusterRoleBindingTargetReader, Class: component.Application, MutateFn: func() {
			v.reconcileGeneralClusterRoleBindingTargetReader(clusterRoleBindingTargetReader, clusterRoleTargetReader)
		}},
		{Obj: mutatingWebhookConfguration, Class: component.Application, MutateFn: func() { v.reconcileGeneralMutatingWebhookConfiguration(mutatingWebhookConfguration) }},
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
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch", "create"},
		},
		{
			APIGroups: []string{"poc.autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch", "patch"},
		},
		{
			APIGroups: []string{"autoscaling.k8s.io"},
			Resources: []string{"verticalpodautoscalers"},
			Verbs:     []string{"get", "list", "watch", "patch"},
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
			Namespace: v.serviceAccountNamespace(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      updater,
			Namespace: v.serviceAccountNamespace(),
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
			Namespace: v.serviceAccountNamespace(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      recommender,
			Namespace: v.serviceAccountNamespace(),
		},
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      updater,
			Namespace: v.serviceAccountNamespace(),
		},
	}
}

func (v *vpa) reconcileGeneralMutatingWebhookConfiguration(mutatingWebhookConfiguration *admissionregistrationv1.MutatingWebhookConfiguration) {
	var (
		failurePolicy      = admissionregistrationv1.Ignore
		matchPolicy        = admissionregistrationv1.Exact
		reinvocationPolicy = admissionregistrationv1.NeverReinvocationPolicy
		sideEffects        = admissionregistrationv1.SideEffectClassNone
		scope              = admissionregistrationv1.AllScopes

		clientConfig = admissionregistrationv1.WebhookClientConfig{
			CABundle: v.caBundle,
		}
	)

	if v.values.ClusterType == component.ClusterTypeSeed {
		clientConfig.Service = &admissionregistrationv1.ServiceReference{
			Name:      admissionControllerServiceName,
			Namespace: v.namespace,
			Port:      pointer.Int32(admissionControllerServicePort),
		}
	} else if v.values.ClusterType == component.ClusterTypeShoot {
		// the port is only respected if register-by-url is true, that's why it's in this if-block
		// if it's false it will not set the port during registration, i.e., it will be defaulted to 443,
		// so the servicePort has to be 443 in this case
		// see https://github.com/kubernetes/autoscaler/blob/master/vertical-pod-autoscaler/pkg/admission-controller/config.go#L70-L74
		clientConfig.URL = pointer.String(fmt.Sprintf("https://%s.%s:%d", admissionControllerServiceName, v.namespace, admissionControllerServicePort))
	}

	metav1.SetMetaDataLabel(&mutatingWebhookConfiguration.ObjectMeta, v1beta1constants.LabelExcludeWebhookFromRemediation, "true")
	mutatingWebhookConfiguration.Webhooks = []admissionregistrationv1.MutatingWebhook{{
		Name:                    "vpa.k8s.io",
		AdmissionReviewVersions: []string{"v1"},
		ClientConfig:            clientConfig,
		FailurePolicy:           &failurePolicy,
		MatchPolicy:             &matchPolicy,
		ReinvocationPolicy:      &reinvocationPolicy,
		SideEffects:             &sideEffects,
		TimeoutSeconds:          pointer.Int32(10),
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
