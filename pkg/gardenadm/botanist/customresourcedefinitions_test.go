// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
				HaveField("ObjectMeta.Name", "alertmanagerconfigs.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "alertmanagers.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "backupbuckets.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "backupentries.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "bastions.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "clusterfilters.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "clusterfluentbitconfigs.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "clusterinputs.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "clustermultilineparsers.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "clusteroutputs.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "clusterparsers.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "clusters.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "collectors.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "containerruntimes.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "controlplanes.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "dnsrecords.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "etcdcopybackupstasks.druid.gardener.cloud"),
				HaveField("ObjectMeta.Name", "etcds.druid.gardener.cloud"),
				HaveField("ObjectMeta.Name", "extensions.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "filters.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "fluentbitconfigs.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "fluentbits.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "infrastructures.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "machineclasses.machine.sapcloud.io"),
				HaveField("ObjectMeta.Name", "machinedeployments.machine.sapcloud.io"),
				HaveField("ObjectMeta.Name", "machinesets.machine.sapcloud.io"),
				HaveField("ObjectMeta.Name", "machines.machine.sapcloud.io"),
				HaveField("ObjectMeta.Name", "multilineparsers.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "networks.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "operatingsystemconfigs.extensions.gardener.cloud"),
				HaveField("ObjectMeta.Name", "outputs.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "parsers.fluentbit.fluent.io"),
				HaveField("ObjectMeta.Name", "podmonitors.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "probes.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "prometheusagents.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "prometheuses.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "prometheusrules.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "scrapeconfigs.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "servicemonitors.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "thanosrulers.monitoring.coreos.com"),
				HaveField("ObjectMeta.Name", "verticalpodautoscalercheckpoints.autoscaling.k8s.io"),
				HaveField("ObjectMeta.Name", "verticalpodautoscalers.autoscaling.k8s.io"),
				HaveField("ObjectMeta.Name", "workers.extensions.gardener.cloud"),
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
