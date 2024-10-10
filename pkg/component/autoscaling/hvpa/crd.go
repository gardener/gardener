// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hvpa

import (
	"context"
	_ "embed"
	"time"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

//go:embed templates/crd-autoscaling.k8s.io_hvpas.yaml
var crd string

type crdDeployer struct {
	client  client.Client
	applier kubernetes.Applier
}

// NewCRD can be used to deploy the CRD definitions for the HVPA controller.
func NewCRD(client client.Client, applier kubernetes.Applier) component.DeployWaiter {
	return &crdDeployer{
		client:  client,
		applier: applier,
	}
}

func (v *crdDeployer) Deploy(ctx context.Context) error {
	return v.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(crd)), kubernetes.DefaultMergeFuncs)
}

func (v *crdDeployer) Destroy(ctx context.Context) error {
	return client.IgnoreNotFound(v.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(crd))))
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
func (v *crdDeployer) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForCRD)
	defer cancel()

	return retry.Until(timeoutCtx, IntervalWaitForCRD, func(ctx context.Context) (done bool, err error) {
		r := crd
		crd := &v1.CustomResourceDefinition{}

		obj, err := kubernetes.NewManifestReader([]byte(r)).Read()
		if err != nil {
			return retry.SevereError(err)
		}

		if err := v.client.Get(ctx, client.ObjectKeyFromObject(obj), crd); client.IgnoreNotFound(err) != nil {
			return retry.SevereError(err)
		}

		if err := health.CheckCustomResourceDefinition(crd); err != nil {
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
}

// WaitCleanup for destruction to finish and component to be fully removed. crdDeployer does not need to wait for cleanup.
func (v *crdDeployer) WaitCleanup(_ context.Context) error {
	return nil
}
