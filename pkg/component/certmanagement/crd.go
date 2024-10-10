// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certmanagement

import (
	"context"
	_ "embed"
	"time"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	//go:embed assets/crd-cert.gardener.cloud_certificaterevocations.yaml
	crdRevocations string
	//go:embed assets/crd-cert.gardener.cloud_certificates.yaml
	crdCertificates string
	//go:embed assets/crd-cert.gardener.cloud_issuers.yaml
	crdIssuers string

	resources []string
)

func init() {
	resources = []string{crdRevocations, crdCertificates, crdIssuers}
}

type crdDeployer struct {
	client  client.Client
	applier kubernetes.Applier
}

// NewCRDs can be used to deploy the CRD definitions for the cert-management.
func NewCRDs(client client.Client, applier kubernetes.Applier) component.DeployWaiter {
	return &crdDeployer{
		client:  client,
		applier: applier,
	}
}

func (c *crdDeployer) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(r)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (c *crdDeployer) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(r))))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

var (
	// IntervalWaitForCRD is the interval used while waiting for the CRDs to become healthy
	// or deleted.
	IntervalWaitForCRD = 1 * time.Second
	// TimeoutWaitForCRD is the timeout used while waiting for the CRDs to become healthy
	// or deleted.
	TimeoutWaitForCRD = 15 * time.Second
	// Until is an alias for retry.Until. Exposed for tests.
	Until = retry.Until
)

// Wait signals whether a CRD is ready or needs more time to be deployed.
func (c *crdDeployer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForCRD)
	defer cancel()

	return retry.Until(timeoutCtx, IntervalWaitForCRD, func(ctx context.Context) (done bool, err error) {
		for _, resource := range resources {
			r := resource
			crd := &v1.CustomResourceDefinition{}

			obj, err := kubernetes.NewManifestReader([]byte(r)).Read()
			if err != nil {
				return retry.SevereError(err)
			}

			if err := c.client.Get(ctx, client.ObjectKeyFromObject(obj), crd); client.IgnoreNotFound(err) != nil {
				return retry.SevereError(err)
			}

			if err := health.CheckCustomResourceDefinition(crd); err != nil {
				return retry.MinorError(err)
			}
		}
		return retry.Ok()
	})
}

// WaitCleanup for destruction to finish and component to be fully removed. crdDeployer does not need to wait for cleanup.
func (c *crdDeployer) WaitCleanup(_ context.Context) error {
	return nil
}
