// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
}

func newTableConvertor() rest.TableConvertor {
	return &convertor{
		headers: []metav1beta1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "CloudProfile", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["cloudprofile"]},
			{Name: "Provider", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["provider"]},
			{Name: "Region", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["region"]},
			{Name: "Seed", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["seed"], Priority: 1},
			{Name: "K8S Version", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["k8sVersion"]},
			{Name: "Hibernation", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["hibernation"]},
			{Name: "Last Operation", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["lastoperation"]},
			{Name: "Status", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["status"]},

			{Name: "Purpose", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["purpose"], Priority: 1},
			{Name: "Gardener Version", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["gardenerVersion"], Priority: 1},
			{Name: "APIServer", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["apiserver"], Priority: 1},
			{Name: "Control", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["control"], Priority: 1},
			{Name: "Observability", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["observability"], Priority: 1},
			{Name: "Nodes", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["nodes"], Priority: 1},
			{Name: "System", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["system"], Priority: 1},
			{Name: "Age", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		},
	}
}

// ConvertToTable converts the output to a table.
func (c *convertor) ConvertToTable(_ context.Context, obj runtime.Object, _ runtime.Object) (*metav1beta1.Table, error) {
	var (
		err   error
		table = &metav1beta1.Table{
			ColumnDefinitions: c.headers,
		}
	)

	if m, err := meta.ListAccessor(obj); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.Continue = m.GetContinue()
	} else {
		if m, err := meta.CommonAccessor(obj); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
		}
	}

	table.Rows, err = metatable.MetaToTableRow(obj, func(obj runtime.Object, _ metav1.Object, _, _ string) ([]any, error) {
		var (
			shoot = obj.(*core.Shoot)
			cells = []any{}
		)

		cells = append(cells, shoot.Name)

		cloudProfileReference := gardenerutils.BuildCloudProfileReference_core(shoot)
		switch cloudProfileReference.Kind {
		case v1beta1constants.CloudProfileReferenceKindCloudProfile:
			cells = append(cells, cloudProfileReference.Name)
		case v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile:
			cells = append(cells, shoot.Namespace+"/"+cloudProfileReference.Name)
		default:
			cells = append(cells, "")
		}

		cells = append(cells, shoot.Spec.Provider.Type)
		cells = append(cells, shoot.Spec.Region)
		if seed := shoot.Spec.SeedName; seed != nil {
			cells = append(cells, *seed)
		} else {
			cells = append(cells, "<none>")
		}
		cells = append(cells, shoot.Spec.Kubernetes.Version)

		specHibernated := shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled
		statusHibernated := shoot.Status.IsHibernated
		switch {
		case specHibernated && statusHibernated:
			cells = append(cells, "Hibernated")
		case specHibernated && !statusHibernated:
			cells = append(cells, "Hibernating")
		case !specHibernated && statusHibernated:
			cells = append(cells, "Waking Up")
		default:
			cells = append(cells, "Awake")
		}
		if lastOp := shoot.Status.LastOperation; lastOp != nil {
			cells = append(cells, fmt.Sprintf("%s %s (%d%%)", lastOp.Type, lastOp.State, lastOp.Progress))
		} else {
			cells = append(cells, "<pending>")
		}
		status, ok := shoot.Labels[v1beta1constants.ShootStatus]
		if !ok {
			cells = append(cells, "<pending>")
		} else {
			cells = append(cells, status)
		}

		if purpose := shoot.Spec.Purpose; purpose != nil {
			cells = append(cells, string(*purpose))
		} else {
			cells = append(cells, "<none>")
		}
		if len(shoot.Status.Gardener.Version) != 0 {
			cells = append(cells, shoot.Status.Gardener.Version)
		} else {
			cells = append(cells, "<unknown>")
		}
		if cond := helper.GetCondition(shoot.Status.Conditions, core.ShootAPIServerAvailable); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		if cond := helper.GetCondition(shoot.Status.Conditions, core.ShootControlPlaneHealthy); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		if cond := helper.GetCondition(shoot.Status.Conditions, core.ShootObservabilityComponentsHealthy); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		if cond := helper.GetCondition(shoot.Status.Conditions, core.ShootEveryNodeReady); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		if cond := helper.GetCondition(shoot.Status.Conditions, core.ShootSystemComponentsHealthy); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		cells = append(cells, metatable.ConvertToHumanReadableDateType(shoot.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
