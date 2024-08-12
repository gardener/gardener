// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

var _ = Describe("Garden metrics", func() {
	var (
		ctx       context.Context
		k8sClient client.Client
		c         prometheus.Collector
	)

	BeforeEach(func() {
		testScheme := runtime.NewScheme()
		Expect(operatorv1alpha1.AddToScheme(testScheme)).To(Succeed())
		k8sClient = fake.NewClientBuilder().
			WithScheme(testScheme).
			WithStatusSubresource(&operatorv1alpha1.Garden{}).
			Build()

		c = newGardenCollector(k8sClient, logr.Discard())
	})

	It("should collect condition metrics", func() {
		garden := &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}
		Expect(k8sClient.Create(ctx, garden)).To(Succeed())

		garden.Status = operatorv1alpha1.GardenStatus{
			LastOperation: &gardencorev1beta1.LastOperation{
				Type: gardencorev1beta1.LastOperationTypeReconcile,
			},
			Conditions: []gardencorev1beta1.Condition{
				{Type: operatorv1alpha1.RuntimeComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
				{Type: operatorv1alpha1.VirtualComponentsHealthy, Status: gardencorev1beta1.ConditionFalse},
			},
		}
		Expect(k8sClient.Status().Update(ctx, garden)).To(Succeed())

		Expect(
			testutil.CollectAndCompare(c, strings.NewReader(`# HELP gardener_operator_garden_condition Condition state of the Garden. Possible values: -1=Unknown|0=Unhealthy|1=Healthy|2=Progressing
# TYPE gardener_operator_garden_condition gauge
gardener_operator_garden_condition{condition="RuntimeComponentsHealthy",name="foo",operation="Reconcile"} 1
gardener_operator_garden_condition{condition="VirtualComponentsHealthy",name="foo",operation="Reconcile"} 0
`), "gardener_operator_garden_condition"),
		).To(Succeed())
	})
})
