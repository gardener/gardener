// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

var (
	//go:embed crds/templates/crd-druid.gardener.cloud_etcds.yaml
	// CRD holds the etcd custom resource definition template
	CRD string
	//go:embed crds/templates/crd-druid.gardener.cloud_etcdcopybackupstasks.yaml
	etcdCopyBackupsTaskCRD string

	etcdCRDName                = "etcds.druid.gardener.cloud"
	etcdCopyBackupsTaskCRDName = "etcdcopybackupstasks.druid.gardener.cloud"
	crdResources               []string
)

type crd struct {
	client  client.Client
	applier kubernetes.Applier
}

func init() {
	crdResources = append(crdResources, CRD, etcdCopyBackupsTaskCRD)
}

// NewCRD can be used to deploy the CRD definitions for Etcd and EtcdCopyBackupsTask.
func NewCRD(c client.Client, applier kubernetes.Applier) component.DeployWaiter {
	return &crd{
		client:  c,
		applier: applier,
	}
}

// Deploy creates and updates the CRD definitions for Etcd and EtcdCopyBackupsTask.
func (c *crd) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range crdResources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(r)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (c *crd) Destroy(ctx context.Context) error {
	etcdList := &druidv1alpha1.EtcdList{}
	// Need to check for both error types. The DynamicRestMapper can hold a stale cache returning a path to a non-existing api-resource leading to a NotFound error.
	if err := c.client.List(ctx, etcdList); err != nil && !meta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
		return err
	}

	if len(etcdList.Items) > 0 {
		return fmt.Errorf("cannot delete etcd CRDs because there are still druidv1alpha1.Etcd resources left in the cluster")
	}

	if err := gardenerutils.ConfirmDeletion(ctx, c.client, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: etcdCRDName}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	etcdCopyBackupsTaskList := &druidv1alpha1.EtcdCopyBackupsTaskList{}
	if err := c.client.List(ctx, etcdCopyBackupsTaskList); err != nil && !meta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
		return err
	}

	if len(etcdCopyBackupsTaskList.Items) > 0 {
		return fmt.Errorf("cannot delete etcd CRDs because there are still druidv1alpha1.EtcdCopyBackupsTask resources left in the cluster")
	}

	if err := gardenerutils.ConfirmDeletion(ctx, c.client, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: etcdCopyBackupsTaskCRDName}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	var fns []flow.TaskFn

	for _, resource := range crdResources {
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
func (c *crd) Wait(ctx context.Context) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, TimeoutWaitForCRD)
	defer cancel()

	return retry.Until(timeoutCtx, IntervalWaitForCRD, func(ctx context.Context) (done bool, err error) {
		for _, resource := range crdResources {
			r := resource
			crd := &apiextensionsv1.CustomResourceDefinition{}

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
func (c *crd) WaitCleanup(_ context.Context) error {
	return nil
}
