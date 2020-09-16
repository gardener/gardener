// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"

	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"
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
			{Name: "Version", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["version"]},
			{Name: "Seed", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["seed"]},
			{Name: "Domain", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["domain"]},
			{Name: "Hibernation", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["hibernation"]},
			{Name: "Operation", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["operation"]},
			{Name: "Progress", Type: "integer", Format: "name", Description: swaggerMetadataDescriptions["progress"]},
			{Name: "APIServer", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["apiserver"]},
			{Name: "Control", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["control"]},
			{Name: "Nodes", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["nodes"]},
			{Name: "System", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["system"]},
			{Name: "Age", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		},
	}
}

// ConvertToTable converts the output to a table.
func (c *convertor) ConvertToTable(ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1beta1.Table, error) {
	var (
		err   error
		table = &metav1beta1.Table{
			ColumnDefinitions: c.headers,
		}
	)

	if m, err := meta.ListAccessor(obj); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.SelfLink = m.GetSelfLink()
		table.Continue = m.GetContinue()
	} else {
		if m, err := meta.CommonAccessor(obj); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
			table.SelfLink = m.GetSelfLink()
		}
	}

	table.Rows, err = metatable.MetaToTableRow(obj, func(obj runtime.Object, m metav1.Object, name, age string) ([]interface{}, error) {
		var (
			shoot = obj.(*core.Shoot)
			cells = []interface{}{}
		)

		cells = append(cells, shoot.Name)
		cells = append(cells, shoot.Spec.CloudProfileName)
		cells = append(cells, shoot.Spec.Kubernetes.Version)
		if seed := shoot.Spec.SeedName; seed != nil {
			cells = append(cells, *seed)
		} else {
			cells = append(cells, "<none>")
		}
		if shoot.Spec.DNS != nil && shoot.Spec.DNS.Domain != nil {
			cells = append(cells, *shoot.Spec.DNS.Domain)
		} else {
			cells = append(cells, "<none>")
		}
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
			cells = append(cells, lastOp.State)
			cells = append(cells, lastOp.Progress)
		} else {
			cells = append(cells, "<pending>")
			cells = append(cells, 0)
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
