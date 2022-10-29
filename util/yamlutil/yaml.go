package yamlutil

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/embedded"
	"gopkg.in/yaml.v3"
)

// WriteYAML encodes struct to file as YAML.
func WriteYAML(value interface{}, file string) error {
	b, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("error encoding YAML: %w", err)
	}

	return os.WriteFile(file, b, 0644)
}

// Save saves the config.
func Save(c config.Config, file string) error {
	b, err := encodeYAML(c)
	if err != nil {
		return err
	}
	if err := os.WriteFile(file, b, 0644); err != nil {
		return fmt.Errorf("error writing yaml file: %w", err)
	}

	return nil
}

func encodeYAML(conf config.Config) ([]byte, error) {
	var doc yaml.Node

	f, err := embedded.Read("defaults/colima.yaml")
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if err := yaml.Unmarshal(f, &doc); err != nil {
		return nil, fmt.Errorf("embedded default config is invalid yaml: %w", err)
	}

	if l := len(doc.Content); l != 1 {
		return nil, fmt.Errorf("unexpected error during yaml decode: doc has multiple children of len %d", l)
	}
	root := doc.Content[0]

	// get all nodes
	nodeVals := map[string]*yaml.Node{}
	if err := traverseNode("", root, nodeVals); err != nil {
		return nil, fmt.Errorf("error traversing yaml node: %w", err)
	}

	// get all node values
	structVals := map[string]any{}
	traverseConfig("", conf, structVals)

	// apply values to nodes
	for key, node := range nodeVals {
		val := structVals[key]

		// top level, ignore. except known maps.
		if node.Kind == yaml.MappingNode {
			switch val.(type) {
			case map[string]any:
			case map[string]string:

			default:
				continue
			}
		}

		// lazy way, delegate node construction to the yaml library via a roundtrip.
		// no performance concern as only one file is being read
		b, err := yaml.Marshal(val)
		if err != nil {
			return nil, fmt.Errorf("unexpected error nested value encoding: %w", err)
		}
		var newNode yaml.Node
		if err := yaml.Unmarshal(b, &newNode); err != nil {
			return nil, fmt.Errorf("unexpected error during yaml node traversal: %w", err)
		}

		if l := len(newNode.Content); l != 1 {
			return nil, fmt.Errorf("unexpected error during yaml node traversal: doc has multiple children of len %d", l)
		}
		*node = *newNode.Content[0]
	}

	b, err := encode(root)
	if err != nil {
		return nil, fmt.Errorf("error encoding yaml file: %w", err)
	}

	return b, nil
}

func traverseConfig(parentKey string, s any, vals map[string]any) {
	typ := reflect.TypeOf(s)
	val := reflect.ValueOf(s)

	// everything else is a value, no nesting required
	if typ.Kind() != reflect.Struct {
		vals[parentKey] = val.Interface()
		return
	}

	// traverse the struct fields recursively
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		key := strings.TrimSuffix(field.Tag.Get("yaml"), ",omitempty")
		if key == "" || key == "-" { // no yaml tag is present
			continue
		}

		if parentKey != "" {
			key = parentKey + "." + key
		}
		val := val.Field(i)

		traverseConfig(key, val.Interface(), vals)
	}

}

func traverseNode(parentKey string, node *yaml.Node, vals map[string]*yaml.Node) error {
	switch node.Kind {
	case yaml.MappingNode:
		if l := len(node.Content); l%2 != 0 {
			return fmt.Errorf("uneven children of %d found for mapping node", l)
		}
		for i := 0; i < len(node.Content); i += 2 {
			if i > 1 {
				// fix jumbled comments
				if cn := node.Content[i]; cn.HeadComment != "" {
					if strings.Index(cn.HeadComment, "#") == 0 {
						cn.HeadComment = "\n" + cn.HeadComment
					}
				}
			}

			key := node.Content[i].Value
			val := node.Content[i+1]
			if parentKey != "" {
				key = parentKey + "." + key
			}
			vals[key] = val

			if err := traverseNode(key, val, vals); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i := 0; i < len(node.Content); i++ {
			key := strconv.Itoa(i)
			val := node.Content[i]
			if parentKey != "" {
				key = parentKey + "." + key
			}
			vals[key] = val

			if err := traverseNode(key, val, vals); err != nil {
				return err
			}
		}
	}

	// yaml.ScalarNode has nothing to do
	return nil
}

func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	err := enc.Encode(v)
	return buf.Bytes(), err
}
