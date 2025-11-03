package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/flanksource/clicky/api"
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

func (cs ConfigSetting) GetInt() (int, error) {
	unit := 1

	if cs.Unit != nil {
		switch *cs.Unit {
		case "8kb":
			unit = 8 * 1024
		case "kB", "KB":
			unit = KB
		case "MB":
			unit = MB
		case "GB":
			unit = GB
		}
	}

	num, err := strconv.Atoi(cs.Setting)
	if err != nil {
		return 0, err
	}
	return num * unit, nil
}

var KB = 1024
var MB = 1024 * KB
var GB = 1024 * MB

func (cs ConfigSetting) IsBytes() bool {
	if cs.Unit == nil {
		return false
	}
	switch *cs.Unit {
	case "8kb", "kB", "MB", "GB", "KB":
		return true
	}
	return false
}

func (cs ConfigSetting) String() string {
	if cs.IsBytes() {
		val, err := cs.GetInt()
		if err != nil {
			return cs.Setting
		}

		if val%GB == 0 && val >= GB {
			return fmt.Sprintf("%dGB", val/GB)
		} else if val >= MB && val%MB == 0 {
			return fmt.Sprintf("%dMB", val/MB)
		}
		return fmt.Sprintf("%dkB", val/KB)
	}

	return cs.Setting
}

func (cs ConfigSetting) GetBool() (bool, error) {
	if cs.Setting == "" {
		return false, nil
	}
	switch strings.ToLower(cs.Setting) {
	case "on", "true", "yes":
		return true, nil
	case "off", "false", "no":
		return false, nil
	}
	return false, fmt.Errorf("Uknown bool type: %s", cs.Setting)
}

// ConfSettings is a map of configuration parameter names to their full settings
type ConfSettings []ConfigSetting

func (cs ConfSettings) AsMap() map[string]ConfigSetting {
	result := map[string]ConfigSetting{}
	for _, setting := range cs {
		result[setting.Name] = setting
	}
	return result
}

func (cs ConfSettings) Pretty() api.Text {
	t := api.Text{}
	keys := lo.Keys(cs.AsMap())
	sort.StringSlice(keys).Sort()
	for _, name := range keys {
		setting := cs.AsMap()[name]

		t = t.Append(setting.Name).Append("=", "text-muted").Append(setting.String(), "bold").Append(" #", "text-muted")
		if setting.Unit != nil {
			t = t.Append(setting.Setting, "text-muted").Space().Append(*setting.Unit, "text-muted")
		}
		t = t.Append("@ source: "+setting.Source, "text-muted")

		t = t.NewLine()
	}
	return t
}

// ToConf converts ConfSettings to a simple map of name->setting values
func (cs ConfSettings) ToConf() Conf {
	result := Conf{}
	for _, setting := range cs {
		if setting.Setting == "" {
			continue
		}
		result[setting.Name] = setting.String()
	}
	return result
}

func (c Conf) Sorted() []struct{ Key, Value string } {
	var result []struct{ Key, Value string }
	for k, v := range c {
		if v == "" {
			continue
		}
		result = append(result, struct{ Key, Value string }{Key: k, Value: v})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})
	return result
}

func (c Conf) AsArgs() []string {
	args := []string{}
	for _, e := range c.Sorted() {
		if strings.ContainsAny(e.Value, " ") {
			e.Value = "'" + e.Value + "'"
		}
		args = append(args, " --"+e.Key+"="+e.Value)
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

	for _, e := range c.Sorted() {
		sb.WriteString(e.Key + " = " + e.Value + "\n")
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
