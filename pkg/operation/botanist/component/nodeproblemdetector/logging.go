package nodeproblemdetector

import (
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

const (
	loggingParserName = "nodeProblemDetector"
	loggingParser     = `[PARSER]
    Name        ` + loggingParserName + `
    Format      regex
    Regex       ^(?<severity>\w)(?<time>\d{4} [^\s]*)\s+(?<pid>\d+)\s+(?<source>[^ \]]+)\] (?<log>.*)$
    Time_Key    time
    Time_Format %m%d %H:%M:%S.%L
`
	loggingFilter = `[FILTER]
    Name                parser
    Match               kubernetes.*` + deploymentName + `*` + containerName + `*
    Key_Name            log
    Parser              ` + loggingParserName + `
    Reserve_Data        True
`
)

// CentralLoggingConfiguration returns a fluent-bit parser and filter for the node-problem-detector logs.
func CentralLoggingConfiguration() (component.CentralLoggingConfig, error) {
	return component.CentralLoggingConfig{Filters: loggingFilter, Parsers: loggingParser}, nil
}
