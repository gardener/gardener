package bootstrappers

import "time"

type OSCEvent struct {
	Timestamp time.Time `yaml:"timestamp"`
	Type      string    `yaml:"type"`
	Reason    string    `yaml:"reason"`
	Message   string    `yaml:"message"`
}

type OSCEventBundle struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Items      []OSCEvent `yaml:"items"`
}

type OSCEventList struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Items      []OSCEvent `yaml:"items"`
}
