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
			{Name: "Last Operation", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["lastoperation"]},
			{Name: "Provider", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["provider"]},
			{Name: "Region", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["region"]},
			{Name: "Age", Type: "date", Description: swaggerMetadataDescriptions["creationTimestamp"]},
			{Name: "Version", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["version"]},
			{Name: "K8S Version", Type: "string", Format: "name", Description: swaggerMetadataDescriptions["kubernetesVersion"]},
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
			seed  = obj.(*core.Seed)
			cells = []any{}
		)

		gardenletReadyCondition := helper.GetCondition(seed.Status.Conditions, core.SeedGardenletReady)
		backupBucketCondition := helper.GetCondition(seed.Status.Conditions, core.SeedBackupBucketsReady)
		extensionsReadyCondition := helper.GetCondition(seed.Status.Conditions, core.SeedExtensionsReady)
		seedSystemComponentsHealthyCondition := helper.GetCondition(seed.Status.Conditions, core.SeedSystemComponentsHealthy)

		cells = append(cells, seed.Name)
		if gardenletReadyCondition != nil && gardenletReadyCondition.Status == core.ConditionUnknown {
			cells = append(cells, "Unknown")
		} else if (gardenletReadyCondition == nil || gardenletReadyCondition.Status != core.ConditionTrue) ||
			(backupBucketCondition != nil && backupBucketCondition.Status != core.ConditionTrue) ||
			(extensionsReadyCondition == nil || extensionsReadyCondition.Status == core.ConditionFalse || extensionsReadyCondition.Status == core.ConditionUnknown) ||
			(seedSystemComponentsHealthyCondition != nil && (seedSystemComponentsHealthyCondition.Status == core.ConditionFalse || seedSystemComponentsHealthyCondition.Status == core.ConditionUnknown)) {
			cells = append(cells, "NotReady")
		} else {
			cells = append(cells, "Ready")
		}
		if lastOp := seed.Status.LastOperation; lastOp != nil {
			cells = append(cells, fmt.Sprintf("%s %s (%d%%)", lastOp.Type, lastOp.State, lastOp.Progress))
		} else {
			cells = append(cells, "<pending>")
		}
		cells = append(cells, seed.Spec.Provider.Type)
		cells = append(cells, seed.Spec.Provider.Region)
		cells = append(cells, metatable.ConvertToHumanReadableDateType(seed.CreationTimestamp))
		if gardener := seed.Status.Gardener; gardener != nil && len(gardener.Version) > 0 {
			cells = append(cells, gardener.Version)
		} else {
			cells = append(cells, "<unknown>")
		}
		if k8sVersion := seed.Status.KubernetesVersion; k8sVersion != nil {
			cells = append(cells, *k8sVersion)
		} else {
			cells = append(cells, "<unknown>")
		}

		return cells, nil
	})

	return table, err
}
