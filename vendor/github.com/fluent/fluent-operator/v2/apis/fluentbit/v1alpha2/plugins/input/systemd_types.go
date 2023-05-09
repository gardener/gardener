package input

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins"
	"github.com/fluent/fluent-operator/v2/apis/fluentbit/v1alpha2/plugins/params"
)

// +kubebuilder:object:generate:=true

// The Systemd input plugin allows to collect log messages from the Journald daemon on Linux environments. <br />
// **For full documentation, refer to https://docs.fluentbit.io/manual/pipeline/inputs/systemd**
type Systemd struct {
	// Optional path to the Systemd journal directory,
	// if not set, the plugin will use default paths to read local-only logs.
	Path string `json:"path,omitempty"`
	// Specify the database file to keep track of monitored files and offsets.
	DB string `json:"db,omitempty"`
	// Set a default synchronization (I/O) method. values: Extra, Full, Normal, Off.
	// This flag affects how the internal SQLite engine do synchronization to disk,
	// for more details about each option please refer to this section.
	// note: this option was introduced on Fluent Bit v1.4.6.
	// +kubebuilder:validation:Enum:=Extra;Full;Normal;Off
	DBSync string `json:"dbSync,omitempty"`
	// The tag is used to route messages but on Systemd plugin there is an extra functionality:
	// if the tag includes a star/wildcard, it will be expanded with the Systemd Unit file (e.g: host.* => host.UNIT_NAME).
	Tag string `json:"tag,omitempty"`
	// Set a maximum number of fields (keys) allowed per record.
	MaxFields int `json:"maxFields,omitempty"`
	// When Fluent Bit starts, the Journal might have a high number of logs in the queue.
	// In order to avoid delays and reduce memory usage, this option allows to specify the maximum number of log entries that can be processed per round.
	// Once the limit is reached, Fluent Bit will continue processing the remaining log entries once Journald performs the notification.
	MaxEntries int `json:"maxEntries,omitempty"`
	// Allows to perform a query over logs that contains a specific Journald key/value pairs, e.g: _SYSTEMD_UNIT=UNIT.
	// The Systemd_Filter option can be specified multiple times in the input section to apply multiple filters as required.
	SystemdFilter []string `json:"systemdFilter,omitempty"`
	// Define the filter type when Systemd_Filter is specified multiple times. Allowed values are And and Or.
	// With And a record is matched only when all of the Systemd_Filter have a match.
	// With Or a record is matched when any of the Systemd_Filter has a match.
	// +kubebuilder:validation:Enum:=And;Or
	SystemdFilterType string `json:"systemdFilterType,omitempty"`
	// Start reading new entries. Skip entries already stored in Journald.
	// +kubebuilder:validation:Enum:=on;off
	ReadFromTail string `json:"readFromTail,omitempty"`
	// Remove the leading underscore of the Journald field (key). For example the Journald field _PID becomes the key PID.
	// +kubebuilder:validation:Enum:=on;off
	StripUnderscores string `json:"stripUnderscores,omitempty"`
}

func (_ *Systemd) Name() string {
	return "systemd"
}

func (s *Systemd) Params(_ plugins.SecretLoader) (*params.KVs, error) {
	kvs := params.NewKVs()

	if s.Path != "" {
		kvs.Insert("Path", s.Path)
	}
	if s.DB != "" {
		kvs.Insert("DB", s.DB)
	}
	if s.DBSync != "" {
		kvs.Insert("DB.Sync", s.DBSync)
	}
	if s.Tag != "" {
		kvs.Insert("Tag", s.Tag)
	}
	if s.MaxFields > 0 {
		kvs.Insert("Max_Fields", string(rune(s.MaxFields)))
	}
	if s.MaxEntries > 0 {
		kvs.Insert("Max_Entries", string(rune(s.MaxEntries)))
	}
	if s.SystemdFilter != nil && len(s.SystemdFilter) > 0 {
		for _, v := range s.SystemdFilter {
			kvs.Insert("Systemd_Filter", v)
		}
	}
	if s.SystemdFilterType != "" {
		kvs.Insert("Systemd_Filter_Type", s.SystemdFilterType)
	}
	if s.ReadFromTail != "" {
		kvs.Insert("Read_From_Tail", s.ReadFromTail)
	}
	if s.StripUnderscores != "" {
		kvs.Insert("Strip_Underscores", s.StripUnderscores)
	}

	return kvs, nil
}
