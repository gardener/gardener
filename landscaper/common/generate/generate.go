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

package generate

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

// BlueprintImport is an import field in the rendered blueprint
type BlueprintImport struct {
	// Name is the name of the import field
	Name string `json:"name"`
	// Required refines if this import is required
	Required bool `json:"required"`
	// Schema is the JSONSchema of the import field
	Schema BlueprintSchema `json:"schema"`
}

// BlueprintSchema is a JSON schema field in the import field
type BlueprintSchema struct {
	// Type is the type according to JSONSchema of the filed
	Type *string `json:"type,omitempty"`
	// Ref is a reference to the JSONSchema of the field
	// the schemas for fields are written as json files into the /schema directory in the blueprint filesystem
	Ref *string `json:"ref,omitempty"`
	// Description is a description of the field
	Description *string `json:"description,omitempty"`
	// Items are sub-items used when the field is an array type
	Items []BlueprintSchema `json:"items,omitempty"`
}

// RenderBlueprint renders a blueprint filesystem (writes the blueprint + the schema files)
// the `rootOpenAPIDefinitionKey` identifies the key in the OpenAPI definitions that identifies the root
// definition (import configuration of the landscaper component)
// Please note that the function `getOpenAPIDefinitions` must be generated using the `openapi-gen` binary
func RenderBlueprint(blueprintTemplate *template.Template, 
	scheme *runtime.Scheme, 
	rootOpenAPIDefinitionKey string,
	getOpenAPIDefinitions func(ref common.ReferenceCallback) map[string]common.OpenAPIDefinition,
	blueprintDirectory string,
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

	rootOpenAPIDefinition := openAPIDefinitions[rootOpenAPIDefinitionKey]

	if err := renderBlueprint(rootOpenAPIDefinition, blueprintTemplate, blueprintDirectory); err != nil {
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

	// recursively write the blueprint filesystem
	totalFilesWritten, err := writeSchemaDependency(openAPIDefinitions, rootOpenAPIDefinition, namer, blueprintDirectory)
	if err != nil {
		return err
	}

	fmt.Printf("Done writing the blueprint filesystem. Wrote %d files.", totalFilesWritten)

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
func renderBlueprint(definition common.OpenAPIDefinition, blueprintTemplate *template.Template, blueprintDirectory string) error {
	var topLevelFields []BlueprintImport
	requiredFields := sets.NewString(definition.Schema.SchemaProps.Required...)

	// use an alphabetical order for writing the blueprint top-level imports to avoid diffs on re-generation
	keys := sets.NewString()
	for key, _:= range definition.Schema.SchemaProps.Properties {
		keys.Insert(key)
	}
	sort.Strings(keys.List())

	for _, key := range keys.List() {
		// kind is not configurable. It is fixed dependent on which Landscaper component is used
		// For example, for the Landscaper controlplane the kind is "Imports"
		if key == "kind" {
			continue
		}

		schema := definition.Schema.SchemaProps.Properties[key]

		// all object types have a ref directly
		// array types have items which again have refs (default domains, alerting, ...)
		reference := schema.SchemaProps.Ref.Ref.GetURL()

		typeBytes, err := schema.SchemaProps.Type.MarshalJSON()
		if err != nil {
			return err
		}

		fieldType := strings.ReplaceAll(string(typeBytes), "\"", "")

		if  fieldType == "array" {
			if schema.SchemaProps.Items == nil {
				return fmt.Errorf("array type misdefined for key %q", key)
			}

			if schema.SchemaProps.Items.Schema == nil {
				return fmt.Errorf("array type misdefined. No schema defined for key %q", key)
			}

			// we expect that the items of the array are defined as a JSONSchema reference (not inline).
			reference := schema.SchemaProps.Items.Schema.Ref.Ref.GetURL()
			if reference == nil {
				return fmt.Errorf("items of the array type %q muat be defined as a JSONSchema reference (not inline)", key)
			}

			topLevelFields = append(topLevelFields, BlueprintImport{
				Name:     key,
				Required: requiredFields.Has(key),
				Schema:   BlueprintSchema{
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
			topLevelFields = append(topLevelFields, BlueprintImport{
				Name:     key,
				Required: requiredFields.Has(key),
				Schema:   BlueprintSchema{
					Type: &fieldType,
					Description: &description,
				},
			})
			continue
		}

		topLevelFields = append(topLevelFields, BlueprintImport{
			Name:     key,
			Required: requiredFields.Has(key),
			Schema:   BlueprintSchema{
				Ref: pointer.String(reference.String()),
			},
		})
	}

	jsonBytes, err := json.Marshal(topLevelFields)
	if err != nil {
		return err
	}

	yamlOut, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return err
	}

	var ccdScript bytes.Buffer
	if err := blueprintTemplate.Execute(&ccdScript, map[string]string{
		"imports":  string(yamlOut),
	}); err != nil {
		return err
	}

	return ioutil.WriteFile(fmt.Sprintf("%s/blueprint.yaml", blueprintDirectory), ccdScript.Bytes(), 0640)
}

// writeJsonSchemaFile writes a JSONSchema formatted file to the blueprint directory
func writeJsonSchemaFile(filename string, blueprintDirectory string, definition common.OpenAPIDefinition) error {
	importsJSONSchema := definition.Schema

	importsJson, err := importsJSONSchema.MarshalJSON()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(fmt.Sprintf("%s/%s", blueprintDirectory, filename), importsJson, 0640)
}

// getBlueprintFilepathForJSONSchemaReference returns the filepath in the blueprint filesystem for a given schema name
func getBlueprintFilepathForJSONSchemaReference(name string, namer *openapinamer.DefinitionNamer) string {
	defName, _ := namer.GetDefinitionName(name)
	schemaName := common.EscapeJsonPointer(defName)
	filepath := fmt.Sprintf("schema/%s.json", common.EscapeJsonPointer(schemaName))
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
		totalFilesToWrite+= writtenFiles

		filepath := getBlueprintFilepathForJSONSchemaReference(schemaDependency, namer)

		if err := writeJsonSchemaFile(filepath, blueprintDirectory, subDefinition); err != nil {
			return 0, err
		}

		totalFilesToWrite++
	}

	return totalFilesToWrite, nil
}
