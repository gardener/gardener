// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package storage

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"
	corev1 "k8s.io/api/core/v1"
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
			{Name: "Owner", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["owner"]},
			{Name: "Namespace", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["namespace"]},
			{Name: "Status", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["status"]},
			{Name: "Ready", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["ready"]},
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
			project = obj.(*garden.Project)
			cells   = []interface{}{}
		)

		cells = append(cells, project.Name)
		cells = append(cells, project.Spec.Owner.Name)
		if namespace := project.Spec.Namespace; namespace != nil {
			cells = append(cells, *namespace)
		} else {
			cells = append(cells, "<unknown>")
		}
		if project.DeletionTimestamp == nil {
			if project.Spec.Namespace == nil {
				cells = append(cells, "Pending")
			} else {
				cells = append(cells, "Active")
			}
		} else {
			cells = append(cells, "Terminating")
		}
		if cond := helper.GetCondition(project.Status.Conditions, garden.ProjectNamespaceReady); cond != nil {
			if cond.Status == corev1.ConditionTrue {
				cells = append(cells, cond.Status)
			} else {
				cells = append(cells, fmt.Sprintf("%s (%s)", cond.Status, cond.Reason))
			}
		} else {
			cells = append(cells, "<unknown>")
		}
		cells = append(cells, metatable.ConvertToHumanReadableDateType(project.CreationTimestamp))

		return cells, nil
	})

	return table, err
}
