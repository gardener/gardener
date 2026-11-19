// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

type fakeHealthCheck struct {
	// sourceClient is set via injection on the per-reconciliation copy.
	sourceClient client.Client
	// observedClients records the clients seen by Check across all executions, keyed by the copy that ran.
	observedClients map[*fakeHealthCheck]client.Client
}

func (c *fakeHealthCheck) Check(context.Context, types.NamespacedName) (*SingleCheckResult, error) {
	c.observedClients[c] = c.sourceClient
	return &SingleCheckResult{Status: gardencorev1beta1.ConditionTrue}, nil
}

func (c *fakeHealthCheck) SetLoggerSuffix(string, string) {}

func (c *fakeHealthCheck) InjectSourceClient(sourceClient client.Client) {
	c.sourceClient = sourceClient
}

var _ = Describe("actuator", func() {
	Describe("#ExecuteHealthCheckFunctions", func() {
		It("should inject the client into a copy and leave the shared health check untouched", func() {
			var (
				shared = &fakeHealthCheck{observedClients: map[*fakeHealthCheck]client.Client{}}

				sourceClient = fakeclient.NewClientBuilder().Build()

				actuator = &Actuator{
					sourceClient: sourceClient,
					healthChecks: []ConditionTypeToHealthCheck{
						{ConditionType: "ControlPlaneHealthy", HealthCheck: shared},
					},
				}
			)

			_, err := actuator.ExecuteHealthCheckFunctions(context.Background(), logr.Discard(), types.NamespacedName{Namespace: "shoot--foo--bar"})
			Expect(err).NotTo(HaveOccurred())

			Expect(shared.sourceClient).To(BeNil())
			Expect(shared.observedClients).To(HaveLen(1))
			for copied, observed := range shared.observedClients {
				Expect(copied).NotTo(BeIdenticalTo(shared))
				Expect(observed).To(BeIdenticalTo(sourceClient))
			}
		})
	})
})
