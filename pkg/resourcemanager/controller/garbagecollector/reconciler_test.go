// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garbagecollector_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	resourcemanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/resourcemanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
)

var _ = Describe("Collector", func() {
	var (
		ctx = context.TODO()

		c  client.Client
		gc *Reconciler

		minimumObjectLifetime = time.Minute
		creationTimestamp     = metav1.Date(2000, 5, 5, 5, 30, 0, 0, time.Local)
		fakeClock             = testclock.NewFakeClock(creationTimestamp.Add(minimumObjectLifetime / 2))
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		gc = &Reconciler{
			TargetClient:          c,
			Config:                resourcemanagerconfigv1alpha1.GarbageCollectorControllerConfig{SyncPeriod: &metav1.Duration{}},
			Clock:                 fakeClock,
			MinimumObjectLifetime: &minimumObjectLifetime,
		}
	})

	Describe("#collectGarbage", func() {
		var (
			unlabeledSecret    *corev1.Secret
			unlabeledConfigMap *corev1.ConfigMap

			labeledObjectMeta metav1.ObjectMeta

			labeledSecret1       *corev1.Secret
			labeledSecret1System *corev1.Secret
			labeledSecret2       *corev1.Secret
			labeledSecret3       *corev1.Secret
			labeledSecret4       *corev1.Secret
			labeledSecret5       *corev1.Secret
			labeledSecret6       *corev1.Secret
			labeledSecret7       *corev1.Secret
			labeledSecret8       *corev1.Secret
			labeledSecret9       *corev1.Secret

			labeledConfigMap1       *corev1.ConfigMap
			labeledConfigMap1System *corev1.ConfigMap
			labeledConfigMap2       *corev1.ConfigMap
			labeledConfigMap3       *corev1.ConfigMap
			labeledConfigMap4       *corev1.ConfigMap
			labeledConfigMap5       *corev1.ConfigMap
			labeledConfigMap6       *corev1.ConfigMap
			labeledConfigMap7       *corev1.ConfigMap
			labeledConfigMap8       *corev1.ConfigMap
		)

		BeforeEach(func() {
			unlabeledSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unlabeledsecret1",
					Namespace: metav1.NamespaceDefault,
				},
			}
			unlabeledConfigMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unlabeledcm1",
					Namespace: metav1.NamespaceDefault,
				},
			}

			labeledObjectMeta = metav1.ObjectMeta{
				Name:      "labeledobj",
				Namespace: metav1.NamespaceDefault,
				Labels: map[string]string{
					"resources.gardener.cloud/garbage-collectable-reference": "true",
				},
			}

			labeledSecret1 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret1System = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret2 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret3 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret4 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret5 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret6 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret7 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret8 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret9 = &corev1.Secret{ObjectMeta: labeledObjectMeta}
			labeledSecret1.Name += "1"
			labeledSecret1System.Name += "1"
			labeledSecret1System.Namespace = metav1.NamespaceSystem
			labeledSecret2.Name += "2"
			labeledSecret3.Name += "3"
			labeledSecret4.Name += "4"
			labeledSecret5.Name += "5"
			labeledSecret6.Name += "6"
			labeledSecret7.Name += "7"
			labeledSecret8.Name += "8"
			labeledSecret9.Name += "9"
			labeledSecret9.CreationTimestamp = creationTimestamp

			labeledConfigMap1 = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap1System = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap2 = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap3 = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap4 = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap6 = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap7 = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap8 = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap5 = &corev1.ConfigMap{ObjectMeta: labeledObjectMeta}
			labeledConfigMap1.Name += "1"
			labeledConfigMap1System.Name += "1"
			labeledConfigMap1System.Namespace += metav1.NamespaceSystem
			labeledConfigMap2.Name += "2"
			labeledConfigMap3.Name += "3"
			labeledConfigMap4.Name += "4"
			labeledConfigMap5.Name += "9"
			labeledConfigMap5.CreationTimestamp = creationTimestamp
			labeledConfigMap6.Name += "6"
			labeledConfigMap7.Name += "7"
			labeledConfigMap8.Name += "8"
		})

		It("should do nothing because no secrets or configmaps found", func() {
			secretList := &corev1.SecretList{}
			Expect(c.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())

			configMapList := &corev1.ConfigMapList{}
			Expect(c.List(ctx, configMapList)).To(Succeed())
			Expect(configMapList.Items).To(BeEmpty())

			_, err := gc.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())

			secretList = &corev1.SecretList{}
			Expect(c.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())

			configMapList = &corev1.ConfigMapList{}
			Expect(c.List(ctx, configMapList)).To(Succeed())
			Expect(configMapList.Items).To(BeEmpty())
		})

		It("should delete nothing because no labeled secrets or configmaps found", func() {
			Expect(c.Create(ctx, unlabeledSecret)).To(Succeed())
			Expect(c.Create(ctx, unlabeledConfigMap)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(c.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(ConsistOf(*unlabeledSecret))

			configMapList := &corev1.ConfigMapList{}
			Expect(c.List(ctx, configMapList)).To(Succeed())
			Expect(configMapList.Items).To(ConsistOf(*unlabeledConfigMap))

			_, err := gc.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())

			secretList = &corev1.SecretList{}
			Expect(c.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(ConsistOf(*unlabeledSecret))

			configMapList = &corev1.ConfigMapList{}
			Expect(c.List(ctx, configMapList)).To(Succeed())
			Expect(configMapList.Items).To(ConsistOf(*unlabeledConfigMap))
		})

		It("should delete the unused resources", func() {
			Expect(c.Create(ctx, labeledSecret1)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret1System)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret2)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret3)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret4)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret5)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret6)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret7)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret8)).To(Succeed())
			Expect(c.Create(ctx, labeledSecret9)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap1)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap1System)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap2)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap3)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap4)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap5)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap6)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap7)).To(Succeed())
			Expect(c.Create(ctx, labeledConfigMap8)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(c.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(ConsistOf(
				*labeledSecret1, *labeledSecret1System, *labeledSecret2, *labeledSecret3,
				*labeledSecret4, *labeledSecret6, *labeledSecret7, *labeledSecret8,
				*labeledSecret9, *labeledSecret5,
			))

			configMapList := &corev1.ConfigMapList{}
			Expect(c.List(ctx, configMapList)).To(Succeed())
			Expect(configMapList.Items).To(ConsistOf(
				*labeledConfigMap1, *labeledConfigMap1System, *labeledConfigMap2, *labeledConfigMap3,
				*labeledConfigMap4, *labeledConfigMap6, *labeledConfigMap7, *labeledConfigMap8, *labeledConfigMap5,
			))

			Expect(c.Create(ctx, &appsv1.Deployment{ObjectMeta: objectMetaFor("deploy1", labeledSecret1, labeledConfigMap1)})).To(Succeed())
			Expect(c.Create(ctx, &appsv1.StatefulSet{ObjectMeta: objectMetaFor("sts1", labeledSecret2, labeledConfigMap2)})).To(Succeed())
			Expect(c.Create(ctx, &appsv1.DaemonSet{ObjectMeta: objectMetaFor("ds1", labeledSecret3, labeledConfigMap3)})).To(Succeed())
			Expect(c.Create(ctx, &batchv1.Job{ObjectMeta: objectMetaFor("job1", labeledSecret4, labeledConfigMap4)})).To(Succeed())
			Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: objectMetaFor("mr1", labeledSecret5)})).To(Succeed())
			Expect(c.Create(ctx, &corev1.Pod{ObjectMeta: objectMetaFor("pod1", labeledSecret6, labeledConfigMap6)})).To(Succeed())
			Expect(c.Create(ctx, &batchv1.CronJob{ObjectMeta: objectMetaFor("cronjob2", labeledSecret7, labeledConfigMap7)})).To(Succeed())

			_, err := gc.Reconcile(ctx, reconcile.Request{})
			Expect(err).NotTo(HaveOccurred())

			secretList = &corev1.SecretList{}
			Expect(c.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(ConsistOf(
				*labeledSecret1, *labeledSecret2, *labeledSecret3,
				*labeledSecret4, *labeledSecret5, *labeledSecret6,
				*labeledSecret7, *labeledSecret9,
			))

			configMapList = &corev1.ConfigMapList{}
			Expect(c.List(ctx, configMapList)).To(Succeed())
			Expect(configMapList.Items).To(ConsistOf(
				*labeledConfigMap1, *labeledConfigMap2, *labeledConfigMap3,
				*labeledConfigMap4, *labeledConfigMap5, *labeledConfigMap6,
				*labeledConfigMap7,
			))
		})
	})
})

func objectMetaFor(name string, objs ...runtime.Object) metav1.ObjectMeta {
	annotations := make(map[string]string)

	for _, obj := range objs {
		var kind, name string

		switch t := obj.(type) {
		case *corev1.Secret:
			kind, name = references.KindSecret, t.Name
		case *corev1.ConfigMap:
			kind, name = references.KindConfigMap, t.Name
		}

		annotations[references.AnnotationKey(kind, name)] = name
	}

	return metav1.ObjectMeta{
		Name:        name,
		Namespace:   metav1.NamespaceDefault,
		Annotations: annotations,
	}
}
