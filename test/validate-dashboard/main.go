package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

type Dashboard struct {
	Title    string  `json:"title"`
	Timezone string  `json:"timezone"`
	Panels   []Panel `json:"panels"`
	// Add other fields as needed
}

type Panel struct {
	Datasource string `json:"datasource"`
}

type MultiError []error

func (m MultiError) Error() string {
	var errMsgs []string
	for _, err := range m {
		errMsgs = append(errMsgs, err.Error())
	}
	return strings.Join(errMsgs, "\n")
}

func validateDashboard(jsonFilePath string) error {
	jsonFile, err := ioutil.ReadFile(jsonFilePath)

	if err != nil {
		return fmt.Errorf("failed to read JSON file: %v", err)
	}

	//validate the json format
	var jsonData interface{}
	err = json.Unmarshal(jsonFile, &jsonData)
	if err != nil {

		if parseErr, ok := err.(*json.SyntaxError); ok {
			line, col := findLineAndColumn(jsonFile, int(parseErr.Offset))
			return fmt.Errorf("invalid JSON format at line %d, column %d: %v", line, col, err)
		}
		return fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Unmarshal the JSON into a Dashboard struct
	var dashboard Dashboard
	err = json.Unmarshal(jsonFile, &dashboard)
	if err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	var validationErrors MultiError

	if dashboard.Title == "" {
		validationErrors = append(validationErrors, fmt.Errorf("dashboard title is empty"))
	}

	for i, panel := range dashboard.Panels {
		if panel.Datasource != "loki" {
			validationErrors = append(validationErrors, fmt.Errorf("datasource type is not loki for panel %d", i+1))
		}
	}

	if len(validationErrors) > 0 {
		return validationErrors
	}

	return nil

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

	if len(os.Args) < 2 {
		log.Fatal("Please provider the path to the json file as an argument")
	}
	jsonFilePath := os.Args[1]

	err := validateDashboard(jsonFilePath)

	if err != nil {
		if MultiErr, ok := err.(MultiError); ok {
			for _, err := range MultiErr {
				log.Println("Dashboard validation error:", err)
			}
		} else {
			log.Println("Dashboard validation error:", err)
		}
	} else {
		fmt.Println("Dashboard validation passed!")
	}

}
