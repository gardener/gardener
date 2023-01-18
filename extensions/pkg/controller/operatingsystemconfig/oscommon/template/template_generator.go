// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package template

import (
	"bytes"
	"fmt"
	"path"
	"text/template"

	"github.com/go-logr/logr"

	"github.com/gardener/gardener/extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// DefaultUnitsPath is the default CoreOS path where to store units at.
const DefaultUnitsPath = "/etc/systemd/system"

type fileData struct {
	Path              string
	Content           string
	Dirname           string
	Permissions       *string
	TransmitUnencoded *bool
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
func (t *CloudInitGenerator) Generate(_ logr.Logger, data *generator.OperatingSystemConfig) ([]byte, *string, error) {
	var tFiles []*fileData
	for _, file := range data.Files {
		tFile := &fileData{
			Path:              file.Path,
			Dirname:           path.Dir(file.Path),
			TransmitUnencoded: file.TransmitUnencoded,
		}
		if file.TransmitUnencoded != nil && *file.TransmitUnencoded {
			tFile.Content = string(file.Content)
		} else {
			tFile.Content = utils.EncodeBase64(file.Content)
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
