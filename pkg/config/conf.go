package config

import (
	"sort"
	"strings"

	"github.com/samber/lo"
)

type Conf map[string]string

// ConfigSetting represents a single PostgreSQL configuration parameter
// with all metadata from pg_settings system view
type ConfigSetting struct {
	Name           string   `json:"name,omitempty"`
	Setting        string   `json:"setting,omitempty"`
	Unit           *string  `json:"unit,omitempty"`
	Category       string   `json:"category,omitempty"`
	ShortDesc      string   `json:"short_desc,omitempty"`
	ExtraDesc      *string  `json:"extra_desc,omitempty"`
	Context        string   `json:"context,omitempty"`
	Vartype        string   `json:"vartype,omitempty"`
	Source         string   `json:"source,omitempty"`
	MinVal         *string  `json:"min_val,omitempty"`
	MaxVal         *string  `json:"max_val,omitempty"`
	Enumvals       []string `json:"enumvals,omitempty"`
	BootVal        string   `json:"boot_val,omitempty"`
	ResetVal       string   `json:"reset_val,omitempty"`
	Sourcefile     *string  `json:"sourcefile,omitempty"`
	Sourceline     *int     `json:"sourceline,omitempty"`
	PendingRestart bool     `json:"pending_restart,omitempty"`
}

// ConfSettings is a map of configuration parameter names to their full settings
type ConfSettings []ConfigSetting

// ToMap converts ConfSettings to a simple map of name->setting values
func (cs ConfSettings) ToMap() Conf {
	result := Conf{}
	for _, setting := range cs {
		result[setting.Name] = setting.Setting
	}
	return result
}

func (c Conf) AsArgs() []string {
	args := []string{}
	// sort keys for consistent output
	keys := lo.Keys(c)
	sort.StringSlice(keys).Sort()
	for _, k := range keys {
		v := c[k]
		if v == "" {
			continue
		}
		if strings.ContainsAny(v, " ") {
			v = "'" + v + "'"
		}
		args = append(args, " --"+k+"="+v)
	}
	return args
}

func (c Conf) MergeFrom(other Conf) Conf {
	merged := Conf{}
	for k, v := range c {
		merged[k] = v
	}
	for k, v := range other {
		merged[k] = v
	}
	return merged
}

func (c Conf) AsFile() string {
	var sb strings.Builder
	for k, v := range c {
		sb.WriteString(k + " = " + v + "\n")
	}

	return sb.String()
}

// GetPgTuneManagedParams returns the list of parameters managed by pg_tune
var PerformanceParams = []string{
	"checkpoint_completion_target",
	"default_statistics_target",
	"effective_cache_size",
	"effective_io_concurrency",
	"huge_pages",
	"maintenance_work_mem",
	"max_connections",
	"max_parallel_maintenance_workers",
	"max_parallel_workers_per_gather",
	"max_parallel_workers",
	"max_wal_senders",
	"max_wal_size",
	"max_worker_processes",
	"min_wal_size",
	"random_page_cost",
	"shared_buffers",
	"wal_buffers",
	"wal_level",
	"work_mem",
}

var StartupParams = []string{
	"listen_addresses",
	"port",
	"max_wal_size",
	"shared_buffers",
	"wal_level",
	"config-file",
	"D",
	"data-directory",
	"ssl",
}

// Locale-related parameters (applicable to initdb)
var LocaleParams = []string{
	"lc_collate",
	"lc_ctype",
	"lc_messages",
	"lc_monetary",
	"lc_numeric",
	"lc_time",
}

func (c Conf) Core() Conf {
	coreParams := Conf{}

	for k, v := range c {
		if lo.Contains(PerformanceParams, k) ||
			lo.Contains(StartupParams, k) ||
			strings.HasSuffix(k, "_log") {
			continue
		}
		coreParams[k] = v
	}
	return coreParams
}

// ForInitDB filters configuration to only include parameters applicable to initdb
// Returns locale and encoding settings needed for cluster initialization
func (c Conf) ForInitDB() Conf {
	initdbParams := Conf{}

	for _, param := range LocaleParams {
		if val, ok := c[param]; ok && val != "" {
			initdbParams[param] = val
		}
	}

	// Encoding (server_encoding from pg_database)
	if encoding, ok := c["server_encoding"]; ok && encoding != "" {
		initdbParams["encoding"] = encoding
	}

	return initdbParams
}

func (c Conf) ForTempServer() Conf {
	tempServerParams := Conf{}

	for k, v := range c {
		if lo.Contains(StartupParams, k) {
			tempServerParams[k] = v
		}
	}
	tempServerParams["listen_addresses"] = ""

	return tempServerParams
}

// AsInitDBArgs converts initdb-applicable parameters to command-line arguments
func (c Conf) AsInitDBArgs() []string {
	args := []string{}

	// Map parameter names to initdb flags
	paramMap := map[string]string{
		"encoding":    "--encoding",
		"lc_collate":  "--lc-collate",
		"lc_ctype":    "--lc-ctype",
		"lc_messages": "--lc-messages",
		"lc_monetary": "--lc-monetary",
		"lc_numeric":  "--lc-numeric",
		"lc_time":     "--lc-time",
	}

	// Sort keys for consistent output
	keys := lo.Keys(c)
	sort.StringSlice(keys).Sort()

	for _, key := range keys {
		if flag, ok := paramMap[key]; ok {
			value := c[key]
			if value == "" {
				continue
			}
			args = append(args, flag+"="+value)
		}
	}

	return args
}
