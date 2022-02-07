// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package blueprint

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/kube-openapi/pkg/common"
	openapispec "k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/yaml"
)

const (
	// targetType is the identifier for the target of type Kubernetes cluster
	targetType = "landscaper.gardener.cloud/kubernetes-cluster"
	// targetSchema is the identifier for a JSONSchema which should be replaced by a target type
	targetSchema = "com.github.gardener.landscaper.apis.core.v1alpha1.Target.yaml"
	// defaultExportExecutionFileName is the default name for the export execution file name
	defaultExportExecutionFileName = "deploy-execution.yaml"
	// defaultContainerDeployerName is the default name used for the container deployer when rendering the export executions
	defaultContainerDeployerName = "default"
	// typeTarget is the type of field (either in the import or export section of the blueprint)
	typeTarget = "target"
)

// BlueprintField is a field (import or export) in the rendered blueprint
type BlueprintField struct {
	// Name is the name of the import field
	Name string `json:"name"`
	// TargetType is the name of the target import/export
	TargetType string `json:"targetType,omitempty"`
	// Type is the type of field (data / target)
	Type *string `json:"type,omitempty"`
	// Required refines if this import is required
	Required bool `json:"required"`
	// Schema is the JSONSchema of the import field
	Schema *BlueprintSchema `json:"schema,omitempty"`
}

// BlueprintSchema is a JSON schema field in the field
type BlueprintSchema struct {
	// Type is the type according to JSONSchema of the field
	Type *string `json:"type,omitempty"`
	// Ref is a reference to the JSONSchema of the field
	// the schemas for fields are written as json files into the /schema directory in the blueprint filesystem
	Ref *string `json:"ref,omitempty"`
	// Description is a description of the field
	Description *string `json:"description,omitempty"`
	// Items are sub-items used when the field is an array type
	Items []BlueprintSchema `json:"items,omitempty"`
}

// ExportExecutions are the export executions of blueprint corresponding to the exported fields
type ExportExecutions struct {
	// Exports are the exported fields
	Exports map[string]string `json:"exports,omitempty"`
}

// RenderBlueprint renders a blueprint filesystem (writes the blueprint + the schema files)
// Parameters:
//  - `rootImportOpenAPIDefinitionKey` identifies the key in the OpenAPI definitions that identifies the root
//    import definition (import configuration of the landscaper component)
//  - `rootExportOpenAPIDefinitionKey` optionally identifies the key in the OpenAPI definitions that identifies export definition
//  - `exportExecutionFileName` is the optional name of the file in the blueprint filesystem where the export executions are rendered to. This will use GoTemplate!
//  - `containerDeployerName` is the name of the deployer the exports refer to. This has to match the definition in your blueprint template!
//
//     Limitations:
//       - just supports getting export data from one deployer, while the landscaper supports exports from various sub-installations, too.
//       - no support for targets in export executions
// Please note, the function `getOpenAPIDefinitions` should be generated using the `openapi-gen` binary
func RenderBlueprint(blueprintTemplate *template.Template,
	scheme *runtime.Scheme,
	rootImportOpenAPIDefinitionKey string,
	rootExportOpenAPIDefinitionKey *string,
	getOpenAPIDefinitions func(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition,
	blueprintDirectory string,
	exportExecutionFileName *string,
	containerDeployerName *string,
) error {
	namer := openapinamer.NewDefinitionNamer(scheme)

	openAPIDefinitions := getOpenAPIDefinitions(func(name string) openapispec.Ref {
		// construct the references so that they can be resolved in the blueprint virtual filesystem
		// For example: blueprint://schema/com.github.gardener.gardener.landscaper.pkg.controlplane.apis.imports.v1alpha1.VirtualGarden.json
		// "blueprint://schemas/resource-requirements.json"
		//  - directory: schema
		//  - filename: com.github.gardener.gardener.landscaper.pkg.controlplane.apis.imports.v1alpha1.VirtualGarden.json
		filepath := getBlueprintFilepathForJSONSchemaReference(name, namer)
		return openapispec.MustCreateRef(fmt.Sprintf("blueprint://%s", filepath))
	})

	rootImportOpenAPIDefinition := openAPIDefinitions[rootImportOpenAPIDefinitionKey]

	var rootExportOpenAPIDefinition *common.OpenAPIDefinition
	if rootExportOpenAPIDefinitionKey != nil {
		definiton, ok := openAPIDefinitions[*rootExportOpenAPIDefinitionKey]
		if !ok {
			return fmt.Errorf("openAPI definition for export type %q not found", *rootExportOpenAPIDefinitionKey)
		}
		rootExportOpenAPIDefinition = &definiton
	}

	if err := renderBlueprint(rootImportOpenAPIDefinition, rootExportOpenAPIDefinition, blueprintTemplate, blueprintDirectory, exportExecutionFileName, containerDeployerName); err != nil {
		return err
	}

	blueprintSchemaDirectory := fmt.Sprintf("%s/schema", blueprintDirectory)
	if err := os.MkdirAll(blueprintSchemaDirectory, 0755); err != nil {
		return err
	}

	err := cleanDirectory(blueprintSchemaDirectory)
	if err != nil {
		return err
	}

	// recursively write the blueprint filesystem for schemas used in imports
	totalFilesWritten, err := writeSchemaDependency(openAPIDefinitions, rootImportOpenAPIDefinition, namer, blueprintDirectory)
	if err != nil {
		return err
	}

	// recursively write the blueprint filesystem for schemas used in exports
	if rootExportOpenAPIDefinition != nil {
		totalFilesWrittenExports, err := writeSchemaDependency(openAPIDefinitions, *rootExportOpenAPIDefinition, namer, blueprintDirectory)
		if err != nil {
			return err
		}
		totalFilesWritten += totalFilesWrittenExports
	}

	fmt.Printf("Done writing the blueprint filesystem. Wrote %d files. \n", totalFilesWritten)

	return nil
}

// cleanDirectory deletes all files from a given directory
func cleanDirectory(directory string) error {
	// clear the files in the blueprint filesystem
	dir, err := os.ReadDir(directory)
	if err != nil {
		return err
	}

	for _, d := range dir {
		if err := os.RemoveAll(path.Join([]string{directory, d.Name()}...)); err != nil {
			return err
		}
	}
	return nil
}

// renderBlueprint creates the blueprint.yaml in the blueprint directory based on the `blueprintTemplate`
// The blueprint resource must contain fields defining its import fields. Only using a `ref` to a JSONSchema file is not possible.
// Hence, this function takes the properties of the root schema and renders them as top-level import fields to the blueprint.
func renderBlueprint(importDefinition common.OpenAPIDefinition, exportDefinition *common.OpenAPIDefinition, blueprintTemplate *template.Template, blueprintDirectory string, exportExecutionFileName *string, containerDeployerName *string) error {
	importsFields, _, err := getTopLevelFields(importDefinition)
	if err != nil {
		return err
	}

	data := map[string]string{
		"imports": string(importsFields),
	}

	if exportDefinition != nil {
		exportsFields, exportedFields, err := getTopLevelFields(*exportDefinition)
		if err != nil {
			return err
		}
		data["exports"] = string(exportsFields)

		err = writeExportExecutions(exportedFields, blueprintDirectory, exportExecutionFileName, containerDeployerName)
		if err != nil {
			return err
		}
	}

	var ccdScript bytes.Buffer
	if err := blueprintTemplate.Execute(&ccdScript, data); err != nil {
		return err
	}

	return os.WriteFile(fmt.Sprintf("%s/blueprint.yaml", blueprintDirectory), ccdScript.Bytes(), 0640)
}

// writeExportExecutions writes a file to the blueprint filesystem containing the export executions corresponding to the exported fields.
// Export executions are necessary to "tell" the landscaper from where (deployer) the valus for the exported fields are actually coming from.
// The generator assumes the field name of the exported field matches the name of the field actually exported by the deployer.
// Hence, if your component writes an export.yaml to the export directory with "myCa": "abc", then the exported field name must be "myCa".
// Please note: The generator currently does NOT support exports of type target (but can be added if needed).
func writeExportExecutions(exportedFields []BlueprintField, blueprintDirectory string, exportExecutionFileName *string, containerDeployerName *string) error {
	if exportExecutionFileName == nil {
		exportExecutionFileName = pointer.String(defaultExportExecutionFileName)
	}

	if containerDeployerName == nil {
		containerDeployerName = pointer.String(defaultContainerDeployerName)
	}

	fields := make(map[string]string, len(exportedFields))
	for _, field := range exportedFields {
		fields[field.Name] = fmt.Sprintf(`{{- index .values "deployitems" "%s" "%s" | toYaml | nindent 4 }}`, *containerDeployerName, field.Name)
	}

	exportExecutions := ExportExecutions{Exports: fields}

	importsJson, err := json.Marshal(exportExecutions)
	if err != nil {
		return err
	}

	yamlOut, err := yaml.JSONToYAML(importsJson)
	if err != nil {
		return err
	}

	return os.WriteFile(fmt.Sprintf("%s/%s", blueprintDirectory, *exportExecutionFileName), yamlOut, 0640)
}

func getTopLevelFields(importDefinition common.OpenAPIDefinition) ([]byte, []BlueprintField, error) {
	var topLevelFields []BlueprintField
	requiredFields := sets.NewString(importDefinition.Schema.SchemaProps.Required...)

	// use an alphabetical order for writing the blueprint top-level imports to avoid diffs on re-generation
	keys := sets.NewString()
	for key, _ := range importDefinition.Schema.SchemaProps.Properties {
		keys.Insert(key)
	}
	sort.Strings(keys.List())

	for _, key := range keys.List() {
		// kind is not configurable. It is fixed dependent on which Landscaper component is used
		// For example, for the Landscaper controlplane the kind is "Imports"
		if key == "kind" {
			continue
		}

		schema := importDefinition.Schema.SchemaProps.Properties[key]

		// all object types have a ref directly
		// array types have items which again have refs (default domains, alerting, ...)
		reference := schema.SchemaProps.Ref.Ref.GetURL()

		typeBytes, err := schema.SchemaProps.Type.MarshalJSON()
		if err != nil {
			return nil, nil, err
		}

		fieldType := strings.ReplaceAll(string(typeBytes), "\"", "")

		if fieldType == "array" {
			if schema.SchemaProps.Items == nil {
				return nil, nil, fmt.Errorf("array type misdefined for key %q", key)
			}

			if schema.SchemaProps.Items.Schema == nil {
				return nil, nil, fmt.Errorf("array type misdefined. No schema defined for key %q", key)
			}

			// we expect that the items of the array are defined as a JSONSchema reference (not inline).
			reference := schema.SchemaProps.Items.Schema.Ref.Ref.GetURL()
			if reference == nil {
				return nil, nil, fmt.Errorf("items of the array type %q muat be defined as a JSONSchema reference (not inline)", key)
			}

			topLevelFields = append(topLevelFields, BlueprintField{
				Name:     key,
				Required: requiredFields.Has(key),
				Schema: &BlueprintSchema{
					Type: pointer.String("array"),
					Items: []BlueprintSchema{
						{Ref: pointer.String(reference.String())},
					},
				},
			})

			continue
		}

		// this is a simple field (no schema reference)
		if reference == nil {
			description := strings.ReplaceAll(schema.SchemaProps.Description, "'", "")
			topLevelFields = append(topLevelFields, BlueprintField{
				Name:     key,
				Required: requiredFields.Has(key),
				Schema: &BlueprintSchema{
					Type:        &fieldType,
					Description: &description,
				},
			})
			continue
		}

		if strings.Contains(reference.String(), targetSchema) {
			topLevelFields = append(topLevelFields, BlueprintField{
				Name:       key,
				Required:   requiredFields.Has(key),
				TargetType: targetType,
				Type:       pointer.String(typeTarget),
			})
			continue
		}

		topLevelFields = append(topLevelFields, BlueprintField{
			Name:     key,
			Required: requiredFields.Has(key),
			Schema: &BlueprintSchema{
				Ref: pointer.String(reference.String()),
			},
		})
	}

	jsonBytes, err := json.Marshal(topLevelFields)
	if err != nil {
		return nil, nil, err
	}

	yamlOut, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return nil, nil, err
	}
	return yamlOut, topLevelFields, nil
}

// writeJsonSchemaFile writes a JSONSchema formatted file to the blueprint directory
func writeJsonSchemaFile(filename string, blueprintDirectory string, definition common.OpenAPIDefinition) error {
	importsJSONSchema := definition.Schema

	importsJson, err := importsJSONSchema.MarshalJSON()
	if err != nil {
		return err
	}

	yamlOut, err := yaml.JSONToYAML(importsJson)
	if err != nil {
		return err
	}

	return os.WriteFile(fmt.Sprintf("%s/%s", blueprintDirectory, filename), yamlOut, 0640)
}

// getBlueprintFilepathForJSONSchemaReference returns the filepath in the blueprint filesystem for a given schema name
func getBlueprintFilepathForJSONSchemaReference(name string, namer *openapinamer.DefinitionNamer) string {
	defName, _ := namer.GetDefinitionName(name)
	schemaName := common.EscapeJsonPointer(defName)
	filepath := fmt.Sprintf("schema/%s.yaml", common.EscapeJsonPointer(schemaName))
	return filepath
}

// writeSchemaDependency writes the schema definitions according to the blueprint filesystem
func writeSchemaDependency(allOpenAPIDefinitions map[string]common.OpenAPIDefinition, openAPIDefinition common.OpenAPIDefinition, namer *openapinamer.DefinitionNamer, blueprintDirectory string) (int, error) {
	if len(openAPIDefinition.Dependencies) == 0 {
		return 0, nil
	}

	var totalFilesToWrite int
	for _, schemaDependency := range openAPIDefinition.Dependencies {
		subDefinition, ok := allOpenAPIDefinitions[schemaDependency]
		if !ok {
			// Types that do not generate an OpenAPI spec (with JSONSchema definitions)
			// For the landscaper controlplane this is the case for the following types:
			// - k8s.io/apiserver/pkg/apis/apiserver/v1.AdmissionPluginConfiguration
			// - k8s.io/apiserver/pkg/apis/config/v1.EncryptionConfiguration
			// - github.com/gardener/hvpa-controller/api/v1alpha1.ScaleParams
			// - github.com/gardener/hvpa-controller/api/v1alpha1.ScaleType
			// - github.com/gardener/hvpa-controller/api/v1alpha1.MaintenanceTimeWindowDone

			// replace with reference to a JSONSchema that allows all definitions - reuse AnyJSON
			subDefinition, _ = allOpenAPIDefinitions["github.com/gardener/landscaper/apis/core/v1alpha1.AnyJSON"]
			subDefinition.Schema.SchemaProps.Description = "Allows any json. Used because the original type does not generate any OpenAPI/JSONSchema specification"
		}

		writtenFiles, err := writeSchemaDependency(allOpenAPIDefinitions, subDefinition, namer, blueprintDirectory)
		if err != nil {
			return 0, err
		}
		totalFilesToWrite += writtenFiles

		filepath := getBlueprintFilepathForJSONSchemaReference(schemaDependency, namer)

		if err := writeJsonSchemaFile(filepath, blueprintDirectory, subDefinition); err != nil {
			return 0, err
		}

		totalFilesToWrite++
	}

	return totalFilesToWrite, nil
}
