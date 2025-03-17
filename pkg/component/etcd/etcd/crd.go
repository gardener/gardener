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
	"sigs.k8s.io/yaml"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

type crd struct {
	client       client.Client
	applier      kubernetes.Applier
	crdResources map[string]string
}

// CRDAccess provides methods to manage and get CRDs defined in etcd-druid.
type CRDAccess interface {
	component.Deployer
	CRDGetter
}

// CRDGetter provides methods to get CRDs defined in etcd-druid.
type CRDGetter interface {
	// GetCRDYaml returns the YAML of the CRD with the given name.
	// An error is returned if no CRD is found with the given name.
	GetCRDYaml(name string) (string, error)
	// GetCRD returns the CRD with the given name.
	// An error is returned if no CRD is found with the given name.
	GetCRD(name string) (*apiextensionsv1.CustomResourceDefinition, error)
}

// NewCRDGetter creates a new CRDGetter.
func NewCRDGetter(k8sVersion *semver.Version) (CRDGetter, error) {
	return NewCRD(nil, nil, k8sVersion)
}

// NewCRD can be used to deploy and/or retrieve the CRD definitions for all CRDs defined by etcd-druid.
func NewCRD(c client.Client, applier kubernetes.Applier, k8sVersion *semver.Version) (CRDAccess, error) {
	crdResources, err := getEtcdCRDS(k8sVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get etcd-druid CRDs for Kubernetes version: %s:%w", k8sVersion, err)
	}
	return &crd{
		client:       c,
		applier:      applier,
		crdResources: crdResources,
	}, nil
}

// Deploy creates and updates the CRD definitions for Etcd and EtcdCopyBackupsTask.
func (c *crd) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range c.crdResources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(r)), kubernetes.DefaultMergeFuncs)
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

	for _, resource := range c.crdResources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(r))))
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (c *crd) GetCRDYaml(name string) (string, error) {
	crdYAML, ok := c.crdResources[name]
	if !ok {
		return "", fmt.Errorf("CRD %q not found", name)
	}
	return crdYAML, nil
}

func (c *crd) GetCRD(name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdYAML, err := c.GetCRDYaml(name)
	if err != nil {
		return nil, err
	}
	return getCRDFromYAML(crdYAML)
}

func getEtcdCRDS(k8sVersion *semver.Version) (map[string]string, error) {
	crdYAMLs, err := druidcorecrds.GetAll(k8sVersion.String())
	if err != nil {
		return nil, err
	}
	var crdResources = make(map[string]string, len(crdYAMLs))
	for crdName, crdYAML := range crdYAMLs {
		updatedCrdYamlBytes, err := addDeletionProtectedLabel(crdYAML)
		if err != nil {
			return nil, err
		}
		crdResources[crdName] = string(updatedCrdYamlBytes)
	}
	return crdResources, nil
}

func addDeletionProtectedLabel(crdYAML string) ([]byte, error) {
	crdObj, err := getCRDFromYAML(crdYAML)
	if err != nil {
		return nil, err
	}
	metav1.SetMetaDataLabel(&crdObj.ObjectMeta, gardenerutils.DeletionProtected, "true")
	return yaml.Marshal(crdObj)
}

func getCRDFromYAML(crdYAML string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdObj := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal([]byte(crdYAML), crdObj); err != nil {
		return nil, err
	}
	return crdObj, nil
}
