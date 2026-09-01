// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagerclient "github.com/gardener/gardener/pkg/resourcemanager/client"
)

func TestNetworkPolicyPodSelector(t *testing.T) {
	tests := []struct {
		name    string
		service *corev1.Service
		want    metav1.LabelSelector
		wantErr bool
	}{
		{
			name:    "service selector is used by default",
			service: &corev1.Service{Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "foo"}}},
			want:    metav1.LabelSelector{MatchLabels: map[string]string{"app": "foo"}},
		},
		{
			name: "annotation overrides service selector",
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				resourcesv1alpha1.NetworkingNetworkPolicyPodSelector: `{"matchLabels":{"app":"bar"}}`,
			}}, Spec: corev1.ServiceSpec{Selector: map[string]string{"app": "foo"}}},
			want: metav1.LabelSelector{MatchLabels: map[string]string{"app": "bar"}},
		},
		{
			name: "match expressions are supported",
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				resourcesv1alpha1.NetworkingNetworkPolicyPodSelector: `{"matchExpressions":[{"key":"app","operator":"In","values":["foo","bar"]}]}`,
			}}},
			want: metav1.LabelSelector{MatchExpressions: []metav1.LabelSelectorRequirement{{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"foo", "bar"}}}},
		},
		{
			name: "invalid JSON is rejected",
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				resourcesv1alpha1.NetworkingNetworkPolicyPodSelector: `{`,
			}}},
			wantErr: true,
		},
		{
			name: "invalid selector is rejected",
			service: &corev1.Service{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				resourcesv1alpha1.NetworkingNetworkPolicyPodSelector: `{"matchExpressions":[{"key":"app","operator":"Invalid"}]}`,
			}}},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := networkPolicyPodSelector(test.service)
			if test.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("unexpected selector: got %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestReconcileUsesNetworkPolicyPodSelector(t *testing.T) {
	const (
		namespaceName = "test"
		serviceName   = "server"
	)

	protocol := corev1.ProtocolTCP
	targetPort := intstr.FromInt32(1194)
	targetSelector := metav1.LabelSelector{MatchLabels: map[string]string{"app": "server"}}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: namespaceName,
			Annotations: map[string]string{
				resourcesv1alpha1.NetworkingNetworkPolicyPodSelector: `{"matchLabels":{"app":"server"}}`,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"statefulset.kubernetes.io/pod-name": "server-0"},
			Ports:    []corev1.ServicePort{{Protocol: protocol, TargetPort: targetPort}},
		},
	}
	sourcePodSelector := metav1.LabelSelector{MatchLabels: map[string]string{
		resourcesv1alpha1.NetworkPolicyLabelKeyPrefix + "to-server-tcp-1194": v1beta1constants.LabelNetworkPolicyAllowed,
	}}

	fakeClient := fakeclient.NewClientBuilder().WithScheme(resourcemanagerclient.TargetScheme).WithObjects(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}},
		service,
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: namespaceName, Labels: sourcePodSelector.MatchLabels}},
	).Build()
	reconciler := &Reconciler{TargetClient: fakeClient}

	if _, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: client.ObjectKeyFromObject(service)}); err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}

	ingressPolicy := &networkingv1.NetworkPolicy{}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: namespaceName, Name: "ingress-to-server-tcp-1194"}, ingressPolicy); err != nil {
		t.Fatalf("failed getting ingress policy: %v", err)
	}
	if !reflect.DeepEqual(ingressPolicy.Spec.PodSelector, targetSelector) {
		t.Fatalf("unexpected ingress target selector: got %#v, want %#v", ingressPolicy.Spec.PodSelector, targetSelector)
	}

	egressPolicy := &networkingv1.NetworkPolicy{}
	if err := fakeClient.Get(context.Background(), client.ObjectKey{Namespace: namespaceName, Name: "egress-to-server-tcp-1194"}, egressPolicy); err != nil {
		t.Fatalf("failed getting egress policy: %v", err)
	}
	if !reflect.DeepEqual(egressPolicy.Spec.PodSelector, sourcePodSelector) {
		t.Fatalf("unexpected egress source selector: got %#v, want %#v", egressPolicy.Spec.PodSelector, sourcePodSelector)
	}
	if got := egressPolicy.Spec.Egress[0].To[0].PodSelector; got == nil || !reflect.DeepEqual(*got, targetSelector) {
		t.Fatalf("unexpected egress target selector: got %#v, want %#v", got, targetSelector)
	}
}
