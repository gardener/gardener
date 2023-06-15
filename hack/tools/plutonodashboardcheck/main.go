// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Dashboard struct {
	Title string `json:"title"`
	Uid   string `json:"uid"`
	// Add other fields as needed
}

type MultiError []error

func (m MultiError) Error() string {
	var errMsgs []string
	for _, err := range m {
		errMsgs = append(errMsgs, err.Error())
	}
	return strings.Join(errMsgs, "\n")
}

func validateJSONFile(filePath string) ([]error, error) {
	// Read the JSON file
	jsonFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %v", err)
	}

	//validate the json format
	decoder := json.NewDecoder(bytes.NewReader(jsonFile))
	_, err = decoder.Token()
	if err != nil {

		if parseErr, ok := err.(*json.SyntaxError); ok {
			line, col := findLineAndColumn(jsonFile, int(parseErr.Offset))
			errMsg := fmt.Sprintf("invalid JSON format at line %d, column %d: %v", line, col, err)
			return []error{fmt.Errorf(errMsg)}, nil
		}
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}
	return nil, nil
}

func validateDashboardFile(filePath string) ([]error, error) {
	errs, err := validateJSONFile(filePath)

	if err != nil {
		return nil, err
	}

	//read the json file
	jsonFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %v", err)
	}

	// Unmarshal the JSON into a Dashboard struct
	var dashboard Dashboard
	err = json.Unmarshal(jsonFile, &dashboard)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	errs = append(errs, validateUniqueUID(filePath, dashboard.Uid)...)
	errs = append(errs, validateNonEmptyTitle(filePath, dashboard.Title)...)

	return errs, nil

}

func validateUniqueUID(filePath, uid string) []error {
	var errs []error
	err := filepath.Walk(filepath.Dir(filePath), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if path != filePath && filepath.Ext(path) == ".json" {
			jsonFile, err := ioutil.ReadFile(path)
			if err != nil {
				return fmt.Errorf("failed to read JSON file: %v", err)
			}

			var dashboard Dashboard
			err = json.Unmarshal(jsonFile, &dashboard)
			if err != nil {
				return fmt.Errorf("failed to unmarshal JSON file: %v", err)
			}

			if dashboard.Uid == uid {
				errs = append(errs, fmt.Errorf("duplicate UID found in file : %s", path))
			}
		}
		return nil
	})
	if err != nil {
		errs = append(errs, err)
	}
	return errs
}

func validateNonEmptyTitle(filePath, title string) []error {
	var errs []error
	if title == "" {
		errs = append(errs, fmt.Errorf("empty title found in file: %s", filePath))
	}
	return errs
}

func validateDashboardJSONFiles(folderPath string) {
	errs := make(map[string][]error)

	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("error accessing path: %v", err)
			return nil
		}

		if info.Name() != "gardener" && info.Name() != "dashboards" {
			return filepath.SkipDir
		}

		if info.Name() == "dashboards" && !info.IsDir() && filepath.Ext(path) == ".json" {
			fileErrors, err := validateDashboardFile(path)
			if err != nil {
				log.Printf("Dashboard validation failed: %v", err)
				errs[path] = append(errs[path], err)
			}

			if len(fileErrors) > 0 {
				errs[path] = append(errs[path], fileErrors...)
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("error walking through folder: %v", err)
	}

	if len(errs) > 0 {
		for filePath, fileErrors := range errs {
			log.Printf("Validation errors in file: %s", filePath)
			for _, err := range fileErrors {
				log.Println(err)
			}
			log.Println()
		}
	} else {
		log.Println("All dashboard JSON files are valid.")
	}

}

// findLineAndColumn returns the line and column number for a given offset in a byte array.
func findLineAndColumn(data []byte, offset int) (line, col int) {
	line = 1
	col = 1

	for i, char := range data {
		if i >= offset {
			return line, col
		}

		if char == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	return line, col
}

func main() {

	folderPath := "gardener"
	validateDashboardJSONFiles(folderPath)

}
