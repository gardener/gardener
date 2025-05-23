// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	metatable "k8s.io/apimachinery/pkg/api/meta/table"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
}

func newTableConvertor() rest.TableConvertor {
	return &convertor{
		headers: []metav1beta1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Registration", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["registration"]},
			{Name: "Seed", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["seed"]},
			{Name: "Valid", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["Valid"]},
			{Name: "Installed", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["installed"]},
			{Name: "Healthy", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["healthy"]},
			{Name: "Progressing", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["progressing"]},
			{Name: "Age", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
		},
	}
}

// ConvertToTable converts the output to a table.
func (c *convertor) ConvertToTable(_ context.Context, o runtime.Object, _ runtime.Object) (*metav1beta1.Table, error) {
	var (
		err   error
		table = &metav1beta1.Table{
			ColumnDefinitions: c.headers,
		}
	)

	if m, err := meta.ListAccessor(o); err == nil {
		table.ResourceVersion = m.GetResourceVersion()
		table.Continue = m.GetContinue()
	} else {
		if m, err := meta.CommonAccessor(o); err == nil {
			table.ResourceVersion = m.GetResourceVersion()
		}
	}

	table.Rows, err = metatable.MetaToTableRow(o, func(o runtime.Object, _ metav1.Object, _, _ string) ([]any, error) {
		var (
			obj   = o.(*core.ControllerInstallation)
			cells = []any{}
		)

		cells = append(cells, obj.Name)
		cells = append(cells, obj.Spec.RegistrationRef.Name)
		cells = append(cells, obj.Spec.SeedRef.Name)
		if cond := helper.GetCondition(obj.Status.Conditions, core.ControllerInstallationValid); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		if cond := helper.GetCondition(obj.Status.Conditions, core.ControllerInstallationInstalled); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		if cond := helper.GetCondition(obj.Status.Conditions, core.ControllerInstallationHealthy); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		if cond := helper.GetCondition(obj.Status.Conditions, core.ControllerInstallationProgressing); cond != nil {
			cells = append(cells, cond.Status)
		} else {
			cells = append(cells, "<unknown>")
		}
		cells = append(cells, metatable.ConvertToHumanReadableDateType(obj.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
