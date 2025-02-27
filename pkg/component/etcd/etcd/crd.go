// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	_ "embed"
	"fmt"

	druidcrds "github.com/gardener/etcd-druid/api/core/crds"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var (
	etcdCRDName                = "etcds.druid.gardener.cloud"
	etcdCopyBackupsTaskCRDName = "etcdcopybackupstasks.druid.gardener.cloud"
	crdResources               []string
)

type crd struct {
	client  client.Client
	applier kubernetes.Applier
}

func init() {
	crdResources = append(crdResources, druidcrds.GetEtcdCRD(), druidcrds.GetEtcdCopyBackupsTaskCRD())
}

// NewCRD can be used to deploy the CRD definitions for Etcd and EtcdCopyBackupsTask.
func NewCRD(c client.Client, applier kubernetes.Applier) component.Deployer {
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
