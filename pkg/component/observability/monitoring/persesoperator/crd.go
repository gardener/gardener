// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package persesoperator

import (
	"context"
	_ "embed"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/crddeployer"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	//go:embed templates/crd-perses.dev_perses.yaml
	crdPerses string
	//go:embed templates/crd-perses.dev_persesdashboards.yaml
	crdPersesDashboards string
	//go:embed templates/crd-perses.dev_persesdatasources.yaml
	crdPersesDatasources string
	//go:embed templates/crd-perses.dev_persesglobaldatasources.yaml
	crdPersesGlobalDatasources string

	// TODO(rickardsjp): Remove this variable after v1.141 has been released.
	v1alpha1CRDNames = []string{
		"perses.perses.dev",
		"persesdashboards.perses.dev",
		"persesdatasources.perses.dev",
	}
)

// NewCRDs can be used to deploy perses-operator CRDs.
func NewCRDs(c client.Client) (component.DeployWaiter, error) {
	resources := []string{
		crdPerses,
		crdPersesDashboards,
		crdPersesDatasources,
		crdPersesGlobalDatasources,
	}

	inner, err := crddeployer.New(c, resources, false)
	if err != nil {
		return nil, err
	}

	return &crdDeployerWithCleanup{DeployWaiter: inner, client: c}, nil
}

// TODO(rickardsjp): Remove this struct after v1.141 has been released.
type crdDeployerWithCleanup struct {
	component.DeployWaiter

	client client.Client
}

func (c *crdDeployerWithCleanup) Deploy(ctx context.Context) error {
	// TODO(rickardsjp): Remove this code after v1.141 has been released.
	var deletedCRDs []string

	for _, name := range v1alpha1CRDNames {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: name},
		}

		if err := c.client.Get(ctx, client.ObjectKeyFromObject(crd), crd); err != nil {
			if client.IgnoreNotFound(err) == nil {
				continue
			}
			return err
		}

		if !slices.ContainsFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
			return v.Name == "v1alpha1"
		}) {
			continue
		}

		// Don't need to confirm deletion because Perses CRDs were created without deletion confirmation (and still are).

		if err := c.client.Delete(ctx, crd); client.IgnoreNotFound(err) != nil {
			return err
		}

		deletedCRDs = append(deletedCRDs, name)
	}

	if err := kubernetesutils.WaitUntilCRDManifestsDestroyed(ctx, c.client, deletedCRDs...); err != nil {
		return err
	}

	return c.DeployWaiter.Deploy(ctx)
}
