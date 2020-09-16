// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"fmt"
	"path"
	"text/template"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// DefaultUnitsPath is the default CoreOS path where to store units at.
const DefaultUnitsPath = "/etc/systemd/system"

type fileData struct {
	Path        string
	Content     string
	Dirname     string
	Permissions *string
}

type unitData struct {
	Path    string
	Name    string
	Content *string
	DropIns *dropInsData
}

type dropInsData struct {
	Path  string
	Items []*dropInData
}

type dropInData struct {
	Path    string
	Content string
}

type initScriptData struct {
	CRI              *extensionsv1alpha1.CRIConfig
	Files            []*fileData
	Units            []*unitData
	Bootstrap        bool
	Type             string
	AdditionalValues map[string]interface{}
}

// CloudInitGenerator generates cloud-init scripts.
type CloudInitGenerator struct {
	cloudInitTemplate *template.Template
	unitsPath         string
	cmd               string

	additionalValuesFunc func(*extensionsv1alpha1.OperatingSystemConfig) (map[string]interface{}, error)
}

// Generate generates a cloud-init script from the given OperatingSystemConfig.
func (t *CloudInitGenerator) Generate(data *generator.OperatingSystemConfig) ([]byte, *string, error) {
	var tFiles []*fileData
	for _, file := range data.Files {
		tFile := &fileData{
			Path:    file.Path,
			Content: utils.EncodeBase64(file.Content),
			Dirname: path.Dir(file.Path),
		}
		if file.Permissions != nil {
			permissions := fmt.Sprintf("%04o", *file.Permissions)
			tFile.Permissions = &permissions
		}
		tFiles = append(tFiles, tFile)
	}

	var tUnits []*unitData
	for _, unit := range data.Units {
		var content *string
		if unit.Content != nil {
			encoded := utils.EncodeBase64(unit.Content)
			content = &encoded
		}
		tUnit := &unitData{
			Name:    unit.Name,
			Path:    path.Join(t.unitsPath, unit.Name),
			Content: content,
		}
		if len(unit.DropIns) != 0 {
			dropInPath := path.Join(t.unitsPath, fmt.Sprintf("%s.d", unit.Name))

			var items []*dropInData
			for _, dropIn := range unit.DropIns {
				items = append(items, &dropInData{
					Path:    path.Join(dropInPath, dropIn.Name),
					Content: utils.EncodeBase64(dropIn.Content),
				})
			}
			tUnit.DropIns = &dropInsData{
				Path:  dropInPath,
				Items: items,
			}
		}

		tUnits = append(tUnits, tUnit)
	}

	initScriptData := &initScriptData{
		Type:      data.Object.Spec.Type,
		CRI:       data.CRI,
		Files:     tFiles,
		Units:     tUnits,
		Bootstrap: data.Bootstrap,
	}

	if t.additionalValuesFunc != nil {
		additionalValues, err := t.additionalValuesFunc(data.Object)
		if err != nil {
			return nil, nil, err
		}
		initScriptData.AdditionalValues = additionalValues
	}

	var buf bytes.Buffer
	if err := t.cloudInitTemplate.Execute(&buf, initScriptData); err != nil {
		return nil, nil, err
	}

	var cmd *string
	if data.Path != nil {
		c := fmt.Sprintf(t.cmd, *data.Path)
		cmd = &c
	}

	return buf.Bytes(), cmd, nil
}

// NewCloudInitGenerator creates a new CloudInitGenerator with the given units path.
func NewCloudInitGenerator(template *template.Template, unitsPath string, cmd string, additionalValuesFunc func(*extensionsv1alpha1.OperatingSystemConfig) (map[string]interface{}, error)) *CloudInitGenerator {
	return &CloudInitGenerator{
		cloudInitTemplate:    template,
		unitsPath:            unitsPath,
		cmd:                  cmd,
		additionalValuesFunc: additionalValuesFunc,
	}
}

// NewTemplate creates a new template with the given name.
func NewTemplate(name string) *template.Template {
	return template.New(name).Funcs(template.FuncMap{
		"isContainerDEnabled": func(cri *extensionsv1alpha1.CRIConfig) bool {
			return cri != nil && cri.Name == extensionsv1alpha1.CRINameContainerD
		},
	})
}
