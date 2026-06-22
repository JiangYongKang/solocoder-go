package hotconfig

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"github.com/BurntSushi/toml"
)

type jsonParser struct{}

func (p *jsonParser) Format() ConfigFormat {
	return FormatJSON
}

func (p *jsonParser) Parse(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, &ParseError{
			Format: string(FormatJSON),
			Err:    err,
		}
	}
	return result, nil
}

type yamlParser struct{}

func (p *yamlParser) Format() ConfigFormat {
	return FormatYAML
}

func (p *yamlParser) Parse(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, &ParseError{
			Format: string(FormatYAML),
			Err:    err,
		}
	}
	return convertYAMLMap(result), nil
}

func convertYAMLMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = convertYAMLValue(v)
	}
	return result
}

func convertYAMLValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for mk, mv := range val {
			m[fmt.Sprintf("%v", mk)] = convertYAMLValue(mv)
		}
		return m
	case map[string]interface{}:
		return convertYAMLMap(val)
	case []interface{}:
		arr := make([]interface{}, len(val))
		for i, item := range val {
			arr[i] = convertYAMLValue(item)
		}
		return arr
	default:
		return v
	}
}

type tomlParser struct{}

func (p *tomlParser) Format() ConfigFormat {
	return FormatTOML
}

func (p *tomlParser) Parse(data []byte) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	if err := toml.Unmarshal(data, &result); err != nil {
		return nil, &ParseError{
			Format: string(FormatTOML),
			Err:    err,
		}
	}
	return result, nil
}

var (
	jsonP = &jsonParser{}
	yamlP = &yamlParser{}
	tomlP = &tomlParser{}
)

func GetParser(path string) (Parser, ConfigFormat, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" {
		ext = ext[1:]
	}

	switch ConfigFormat(ext) {
	case FormatJSON:
		return jsonP, FormatJSON, nil
	case FormatYAML, FormatYML:
		return yamlP, FormatYAML, nil
	case FormatTOML:
		return tomlP, FormatTOML, nil
	default:
		return nil, "", fmt.Errorf("%w: %q", ErrUnsupportedFormat, ext)
	}
}

func ParseFile(path string, data []byte) (map[string]interface{}, ConfigFormat, error) {
	parser, format, err := GetParser(path)
	if err != nil {
		return nil, "", err
	}

	result, err := parser.Parse(data)
	if err != nil {
		if pe, ok := err.(*ParseError); ok {
			pe.Path = path
		}
		return nil, "", err
	}
	return result, format, nil
}
