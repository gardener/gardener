// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("CustomResourceDefinitions", func() {
	var (
		ctx context.Context

		fakeClient client.Client

		b *AutonomousBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{apiextensionsv1.SchemeGroupVersion})
		mapper.Add(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"), meta.RESTScopeRoot)
		applier := kubernetes.NewApplier(fakeClient, mapper)

		b = &AutonomousBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					SeedClientSet: fakekubernetes.
						NewClientSetBuilder().
						WithClient(fakeClient).
						WithApplier(applier).
						WithRESTConfig(&rest.Config{}).
						Build(),
					Shoot: &shoot.Shoot{KubernetesVersion: semver.MustParse("1.33.0")},
				},
			},
		}
	})

	Describe("#ReconcileCustomResourceDefinitions", func() {
		It("should reconcile all expected CRDs", func() {
			crdList := &apiextensionsv1.CustomResourceDefinitionList{}
			Expect(fakeClient.List(ctx, crdList)).To(Succeed())

			Expect(b.ReconcileCustomResourceDefinitions(ctx)).To(Succeed())

			Expect(fakeClient.List(ctx, crdList)).To(Succeed())
			Expect(crdList.Items).To(ConsistOf(
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("alertmanagerconfigs.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("alertmanagers.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("backupbuckets.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("backupentries.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("bastions.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterfilters.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterfluentbitconfigs.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterinputs.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clustermultilineparsers.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusteroutputs.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusterparsers.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("clusters.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("collectors.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("containerruntimes.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("controlplanes.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("dnsrecords.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcdcopybackupstasks.druid.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("etcds.druid.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("extensions.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("filters.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluentbitconfigs.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("fluentbits.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("infrastructures.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("multilineparsers.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("networks.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("operatingsystemconfigs.extensions.gardener.cloud")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("outputs.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("parsers.fluentbit.fluent.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("podmonitors.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("probes.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheusagents.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheuses.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("prometheusrules.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("scrapeconfigs.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("servicemonitors.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("thanosrulers.monitoring.coreos.com")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalercheckpoints.autoscaling.k8s.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("verticalpodautoscalers.autoscaling.k8s.io")})}),
				MatchFields(IgnoreExtras, Fields{"ObjectMeta": MatchFields(IgnoreExtras, Fields{"Name": Equal("workers.extensions.gardener.cloud")})}),
			))
		})
	})

	Describe("#EnsureCustomResourceDefinitionsReady", func() {
		var crd *apiextensionsv1.CustomResourceDefinition

		BeforeEach(func() {
			crd = &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: "some.crd"}}
			Expect(fakeClient.Create(ctx, crd)).To(Succeed())
		})

		It("should fail because some CRD is not ready", func() {
			Expect(b.EnsureCustomResourceDefinitionsReady(ctx)).To(MatchError(ContainSubstring(crd.Name + " is not ready yet")))
		})

		It("should succeed because all CRDs are ready", func() {
			crd.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{
				{Type: apiextensionsv1.NamesAccepted, Status: apiextensionsv1.ConditionTrue},
				{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
			}
			Expect(fakeClient.Status().Update(ctx, crd)).To(Succeed())

			Expect(b.EnsureCustomResourceDefinitionsReady(ctx)).To(Succeed())
		})
	})
})
