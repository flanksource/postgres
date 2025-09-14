package main

import (
	"fmt"
	"github.com/knadh/koanf/parsers/yaml"
)

func main() {
	yamlData := []byte(`
postgres:
  max_connections: 100
`)
	parser := yaml.Parser()
	rawData, err := parser.Unmarshal(yamlData)
	fmt.Printf("Type: %T, Error: %v\n", rawData, err)
}
