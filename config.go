package main

import (
	"encoding/json"
	_ "encoding/json"
	"io/ioutil"
	"strings"
)

// ConfigFormat is a configurable format of config file.
type ConfigFormat struct {
	Name      string
	Extension []string
	Loader    ConfigLoader
}

// ConfigLoader is a utility to load config from external source.
type ConfigLoader func(input interface{}) (*Config, error)

var (
	configLoaderByName = make(map[string]*ConfigFormat)
	configLoaderByExt  = make(map[string]*ConfigFormat)
)

// RegisterConfigLoader add a new ConfigLoader.
func RegisterConfigLoader(format *ConfigFormat) error {
	name := strings.ToLower(format.Name)
	if _, found := configLoaderByName[name]; found {
		return newError(format.Name, " already registered.")
	}
	configLoaderByName[name] = format

	for _, ext := range format.Extension {
		lext := strings.ToLower(ext)
		if f, found := configLoaderByExt[lext]; found {
			return newError(ext, " already registered to ", f.Name)
		}
		configLoaderByExt[lext] = format
	}

	return nil
}

func getExtension(filename string) string {
	idx := strings.LastIndexByte(filename, '.')
	if idx == -1 {
		return ""
	}
	return filename[idx+1:]
}

// LoadConfig loads config with given format from given source.
// input accepts 2 different types:
// * []string slice of multiple filename/url(s) to open to read
// * io.Reader that reads a config content (the original way)
func LoadConfig(formatName string, filename string, input interface{}) (*Config, error) {

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, newError("Unable to load config in ", filename).AtWarning()
	}
	config := new(Config)
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}
