package config

import (
	"errors"
	"os"
	"strings"
)

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
