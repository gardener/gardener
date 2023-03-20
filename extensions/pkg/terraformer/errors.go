// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package terraformer

import (
	"regexp"
	"sort"
	"strings"
)

// findTerraformErrors gets the <output> of a Terraform run and parses it to find the occurred
// errors (which will be returned). If no errors occurred, an empty string will be returned.
func findTerraformErrors(output string) string {
	var (
		regexTerraformError = regexp.MustCompile(`(?:Error): *([\s\S]*)`)
		regexUUID           = regexp.MustCompile(`(?i)[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}`)
		regexMultiNewline   = regexp.MustCompile(`\n{2,}`)

		errorMessage = output
		valid        []string
	)

	// Strip optional explanation how Terraform behaves in case of errors.
	if suffixIndex := strings.Index(errorMessage, "\n\nTerraform does not automatically rollback"); suffixIndex != -1 {
		errorMessage = errorMessage[:suffixIndex]
	}
	// Strip optional explanation that nothing will happen.
	if suffixIndex := strings.Index(errorMessage, "\n\nNothing to do."); suffixIndex != -1 {
		errorMessage = errorMessage[:suffixIndex]
	}

	// Search for errors in Terraform output.
	if terraformErrorMatch := regexTerraformError.FindStringSubmatch(errorMessage); len(terraformErrorMatch) > 1 {
		// Remove leading and tailing spaces and newlines.
		errorMessage = strings.TrimSpace(terraformErrorMatch[0])

		// Omit (request) uuid's to allow easy determination of duplicates.
		errorMessage = regexUUID.ReplaceAllString(errorMessage, "<omitted>")

		// Get all errors
		var currentError string
		for _, line := range strings.Split(errorMessage, "\n") {
			if strings.HasPrefix(line, "Error: ") {
				if len(currentError) > 0 {
					valid = append(valid, currentError)
					currentError = ""
				}
				line = strings.TrimPrefix(line, "Error: ")
			}
			currentError += line + "\n"
		}
		if len(currentError) > 0 {
			valid = append(valid, currentError)
		}

		// Sort the occurred errors alphabetically
		sort.Strings(valid)

		errorMessage = "* " + strings.Join(valid, "\n* ")

		// Strip multiple newlines to one newline
		errorMessage = regexMultiNewline.ReplaceAllString(errorMessage, "\n")

		// Remove leading and tailing spaces and newlines.
		errorMessage = strings.TrimSpace(errorMessage)

		return errorMessage
	}
	return ""
}
