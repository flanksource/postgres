package config

import (
	"errors"
	"os"
	"strings"
)

// LoadPostmasterOpts loads postmaster options from a file like:"
// /usr/lib/postgresql/14/bin/postgres "-D" "/var/lib/postgresql/data" "--config-file=/etc/postgresql/postgresql.conf" "--checkpoint_completion_target=0.9" "--db_user_namespace=false" "--effective_cache_size=3GB" "--effective_io_concurrency=200" "--extra_float_digits=0" "--lc_messages=C" "--listen_addresses=*" "--log_autovacuum_min_duration=10s" "--log_checkpoints=true" "--log_connections=false" "--log_destination=stderr" "--log_disconnections=false" "--log_filename=postgresql-%d.log" "--log_line_prefix=%m [%p] %q[user=%u,db=%d,app=%a] " "--log_lock_waits=true" "--log_min_duration_statement=10s" "--log_rotation_age=1d" "--log_rotation_size=100MB" "--log_temp_files=100MB" "--log_timezone=UTC" "--log_truncate_on_rotation=true" "--logging_collector=true" "--maintenance_work_mem=205MB" "--max_connections=100" "--max_parallel_workers=2" "--max_parallel_workers_per_gather=2" "--max_wal_size=3GB" "--max_worker_processes=8" "--min_wal_size=2GB" "--password_encryption=scram-sha-256" "--random_page_cost=1.1" "--shared_buffers=1GB" "--ssl=false" "--timezone=UTC" "--wal_buffers=-1" "--work_mem=10MB"
func LoadPostmasterOpts(path string) (Conf, error) {
	opts := Conf{}
	data, err := os.ReadFile(path)
	if err != nil {
		return opts, err
	}

	for _, arg := range strings.Fields(string(data)) {
		if strings.HasPrefix(arg, "--") {
			parts := strings.SplitN(arg[2:], "=", 2)
			paramName := parts[0]
			paramValue := ""
			if len(parts) > 1 {
				paramValue = parts[1]
			}
			opts[paramName] = paramValue
		}
	}

	return opts, nil

}

func LoadConfFile(path string) (Conf, error) {

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Conf{}, nil
	}
	if err != nil {
		panic(err)
		// return nil, err
	}
	lines := strings.Split(string(data), "\n")
	conf := Conf{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if commentIdx := strings.Index(value, "#"); commentIdx != -1 {
			value = value[:commentIdx]
		}

		value = strings.TrimSpace(value)

		value = strings.Trim(value, "'\"")

		conf[key] = value
	}
	return conf, nil
}
