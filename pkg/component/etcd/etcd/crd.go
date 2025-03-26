// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package etcd

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/Masterminds/semver/v3"
	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	druidcorecrds "github.com/gardener/etcd-druid/api/core/v1alpha1/crds"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

type crd struct {
	client    client.Client
	crdGetter CRDGetter
}

// NewCRD can be used to deploy the CRD definitions for all CRDs defined by etcd-druid.
func NewCRD(c client.Client, k8sVersion *semver.Version) (component.Deployer, error) {
	crdGetter, err := NewCRDGetter(k8sVersion)
	if err != nil {
		return nil, err
	}
	return &crd{
		client:    c,
		crdGetter: crdGetter,
	}, nil
}

var _ component.Deployer = (*crd)(nil)

// Deploy creates and updates the CRD definitions for Etcd and EtcdCopyBackupsTask.
func (c *crd) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range c.crdGetter.GetAllCRDs() {
		r := resource.DeepCopy()
		fns = append(fns, func(ctx context.Context) error {
			_, err := controllerutil.CreateOrPatch(ctx, c.client, r, nil)
			return err
		})
	}
	return flow.Parallel(fns...)(ctx)
}

func (c *crd) Destroy(ctx context.Context) error {
	etcdList := &druidcorev1alpha1.EtcdList{}
	// Need to check for both error types. The DynamicRestMapper can hold a stale cache returning a path to a non-existing api-resource leading to a NotFound error.
	if err := c.client.List(ctx, etcdList); err != nil && !meta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
		return err
	}

	if len(etcdList.Items) > 0 {
		return fmt.Errorf("cannot delete etcd CRDs because there are still druidcorev1alpha1.Etcd resources left in the cluster")
	}

	if err := gardenerutils.ConfirmDeletion(ctx, c.client, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: druidcorecrds.ResourceNameEtcd}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	etcdCopyBackupsTaskList := &druidcorev1alpha1.EtcdCopyBackupsTaskList{}
	if err := c.client.List(ctx, etcdCopyBackupsTaskList); err != nil && !meta.IsNoMatchError(err) && !apierrors.IsNotFound(err) {
		return err
	}

	if len(etcdCopyBackupsTaskList.Items) > 0 {
		return fmt.Errorf("cannot delete etcd CRDs because there are still druidcorev1alpha1.EtcdCopyBackupsTask resources left in the cluster")
	}

	if err := gardenerutils.ConfirmDeletion(ctx, c.client, &apiextensionsv1.CustomResourceDefinition{ObjectMeta: metav1.ObjectMeta{Name: druidcorecrds.ResourceNameEtcdCopyBackupsTask}}); client.IgnoreNotFound(err) != nil {
		return err
	}

	var fns []flow.TaskFn

	for _, resource := range c.crdGetter.GetAllCRDs() {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.client.Delete(ctx, r))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

// CRDGetter provides methods to get CRDs defined in etcd-druid.
type CRDGetter interface {
	// GetAllCRDs returns a map of CRD names to CRD objects.
	GetAllCRDs() map[string]*apiextensionsv1.CustomResourceDefinition
	// GetCRD returns the CRD with the given name.
	// An error is returned if no CRD is found with the given name.
	GetCRD(name string) (*apiextensionsv1.CustomResourceDefinition, error)
}

type crdGetter struct {
	crdResources map[string]*apiextensionsv1.CustomResourceDefinition
}

var _ CRDGetter = (*crdGetter)(nil)

// NewCRDGetter creates a new CRDGetter.
func NewCRDGetter(k8sVersion *semver.Version) (CRDGetter, error) {
	crdResources, err := getEtcdCRDs(k8sVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd-druid CRDs for Kubernetes version %s: %w", k8sVersion, err)
	}
	return &crdGetter{
		crdResources: crdResources,
	}, nil
}

func (c *crdGetter) GetAllCRDs() map[string]*apiextensionsv1.CustomResourceDefinition {
	return c.crdResources
}

func (c *crdGetter) GetCRD(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdObj, ok := c.crdResources[name]
	if !ok {
		return nil, fmt.Errorf("CRD %s not found", name)
	}
	return crdObj, nil
}

func getEtcdCRDs(k8sVersion *semver.Version) (map[string]*apiextensionsv1.CustomResourceDefinition, error) {
	crdYAMLs, err := druidcorecrds.GetAll(k8sVersion.String())
	if err != nil {
		return nil, err
	}
	var crdResources = make(map[string]*apiextensionsv1.CustomResourceDefinition, len(crdYAMLs))
	for crdName, crdYAML := range crdYAMLs {
		crdObj, err := kubernetesutils.DecodeCRD(crdYAML)
		if err != nil {
			return nil, err
		}
		metav1.SetMetaDataLabel(&crdObj.ObjectMeta, gardenerutils.DeletionProtected, "true")
		crdResources[crdName] = crdObj
	}
	return crdResources, nil
}
