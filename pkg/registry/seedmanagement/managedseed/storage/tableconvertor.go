// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
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
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	"github.com/gardener/gardener/pkg/registry/seedmanagement/managedseed"
)

var swaggerMetadataDescriptions = metav1.ObjectMeta{}.SwaggerDoc()

type convertor struct {
	headers []metav1beta1.TableColumnDefinition
}

func newTableConvertor() rest.TableConvertor {
	return &convertor{
		headers: []metav1beta1.TableColumnDefinition{
			{Name: "Name", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["name"]},
			{Name: "Status", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["status"]},
			{Name: "Shoot", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["shootName"]},
			{Name: "Gardenlet", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["gardenlet"]},
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

	table.Rows, err = metatable.MetaToTableRow(obj, func(obj runtime.Object, m metav1.Object, name, age string) ([]interface{}, error) {
		var (
			managedSeed = obj.(*seedmanagement.ManagedSeed)
			cells       = []interface{}{}
		)

		seedRegisteredCondition := helper.GetCondition(managedSeed.Status.Conditions, seedmanagement.ManagedSeedSeedRegistered)

		cells = append(cells, managedSeed.Name)
		if seedRegisteredCondition == nil || seedRegisteredCondition.Status == core.ConditionUnknown {
			cells = append(cells, "Unknown")
		} else if seedRegisteredCondition.Status != core.ConditionTrue {
			cells = append(cells, "NotRegistered")
		} else {
			cells = append(cells, "Registered")
		}
		cells = append(cells, managedseed.GetShootName(managedSeed))
		if managedSeed.Spec.Gardenlet != nil {
			cells = append(cells, "True")
		} else {
			cells = append(cells, "False")
		}
		cells = append(cells, metatable.ConvertToHumanReadableDateType(managedSeed.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
