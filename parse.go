package config

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	arrayTypeRe = regexp.MustCompile(`^array\((.+)\)$`)
	mapTypeRe   = regexp.MustCompile(`^map\((.+)\)$`)
)

var (
	primitiveTypes = map[string]struct{}{
		"boolean":  {},
		"float":    {},
		"integer":  {},
		"string":   {},
		"duration": {},
		"bytes":    {},
		"object":   {},
	}
)

type yamlConfig struct {
	Name        string
	Description string
	Sections    []*yamlSection
}

type yamlSection struct {
	Name        string
	Description string
	URL         string
	Properties  yaml.Node
}

type yamlFile struct {
	Types map[string]*yamlType
}

type yamlType struct {
	Name           string
	Type           string
	Types          []string
	URL            string
	Default        any
	Disabled       bool
	Description    string
	Deprecation    string
	Examples       []*Example
	Aliases        []string
	Reloadable     *bool
	ReloadableNote string `yaml:"reloadable_note"`
	Sections       []*yamlSection
	Properties     yaml.Node
	Version        string
	Choices        []string
}

func (p *yamlType) Combine(b *yamlType) *yamlType {
	x := &(*p)

	if x.Version != "" {
		x.Version = b.Version
	}
	if x.Disabled {
		x.Disabled = b.Disabled
	}
	if x.Description == "" {
		x.Description = b.Description
	}
	if x.Default == nil {
		x.Default = b.Default
	}
	if len(x.Aliases) == 0 {
		x.Aliases = append(x.Aliases, b.Aliases...)
	}
	if x.Reloadable == nil {
		x.Reloadable = b.Reloadable
	}
	if x.ReloadableNote == "" {
		x.ReloadableNote = b.ReloadableNote
	}
	if x.Deprecation == "" {
		x.Deprecation = b.Deprecation
	}
	if x.URL == "" {
		x.URL = b.URL
	}
	if len(x.Examples) == 0 {
		x.Examples = append(x.Examples, b.Examples...)
	}
	if len(x.Choices) == 0 {
		x.Choices = append(x.Choices, b.Choices...)
	}
	// Deep copy sections?
	if len(x.Sections) == 0 {
		x.Sections = append(x.Sections, b.Sections...)
	}

	return x
}

// Parse takes the config and type definition paths and derives the config.
func Parse(path string, typePaths []string) (*Config, error) {
	yc, err := loadConfig(path)
	if err != nil {
		return nil, err
	}

	// Load and index the types for reference when parsing.
	ytypes := make(map[string]*yamlType)
	for _, path := range typePaths {
		f, err := loadTypes(path)
		if err != nil {
			return nil, err
		}
		for k, t := range f.Types {
			// Check for duplicates.
			if _, ok := ytypes[k]; ok {
				return nil, fmt.Errorf("duplicate type found: %q", k)
			}
			t.Name = k
			if t.Type != "" {
				t.Types = []string{t.Type}
				t.Type = ""
			}
			if len(t.Types) == 0 {
				return nil, fmt.Errorf("type %q has no types", k)
			}

			// If this property has properties itself, we define an implicit
			// section for it.
			if !t.Properties.IsZero() {
				if len(t.Sections) > 0 {
					return nil, fmt.Errorf("type %q has both properties and sections", k)
				}

				t.Sections = []*yamlSection{{
					Properties: t.Properties,
				}}

				t.Properties = yaml.Node{}
			}

			ytypes[k] = t
		}
	}

	// Top-level config sections.
	sections, err := parseSections(ytypes, yc.Sections)
	if err != nil {
		return nil, err
	}

	c := Config{
		Name:        yc.Name,
		Description: yc.Description,
		Sections:    sections,
	}

	return &c, nil
}

func loadConfig(path string) (*yamlConfig, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var f yamlConfig
	err = yaml.Unmarshal(b, &f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &f, nil
}

func loadTypes(path string) (*yamlFile, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var f yamlFile
	err = yaml.Unmarshal(b, &f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return &f, nil
}

// parseSections parses a list of encoded YAML sections.
func parseSections(ytypes map[string]*yamlType, yss []*yamlSection) ([]*Section, error) {
	sections := make([]*Section, len(yss))
	for i, ys := range yss {
		s, err := parseSection(ytypes, ys)
		if err != nil {
			return nil, err
		}
		sections[i] = s
	}
	return sections, nil
}

// parseSection parses an encoded YAML section.
func parseSection(ytypes map[string]*yamlType, ys *yamlSection) (*Section, error) {
	// If the section has no properties, it's just a header.
	if len(ys.Properties.Content) == 0 {
		return &Section{
			Name:        ys.Name,
			Description: ys.Description,
		}, nil
	}

	// Validate the node type.
	if ys.Properties.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("expected YAML mapping node: line %d", ys.Properties.Line)
	}

	// Validate there are key-value pairs.
	if len(ys.Properties.Content)%2 != 0 {
		return nil, fmt.Errorf("expected key-value pairs")
	}

	var props []*Property
	for i := 0; i < len(ys.Properties.Content)/2; i++ {
		kc := ys.Properties.Content[i*2]
		vc := ys.Properties.Content[i*2+1]

		// Decode the raw property type info.
		var yp yamlType
		if err := vc.Decode(&yp); err != nil {
			return nil, fmt.Errorf("failed property decode at line %d: %w", vc.Line, err)
		}

		yp.Name = kc.Value

		// Parse the property info to a concrete property.
		p, err := parseProperty(ytypes, &yp)
		if err != nil {
			return nil, err
		}

		props = append(props, p)
	}

	s := Section{
		Name:        ys.Name,
		Description: ys.Description,
		Properties:  props,
	}

	return &s, nil
}

// parseProperty recursively builds a property from the raw property info.
// The provided `type` or `types` dictates how the property is constructed.
// The simplest case is a single primitive type, e.g. `string`.
func parseProperty(ytypes map[string]*yamlType, yp *yamlType) (*Property, error) {
	// Normalize.
	var types []string
	if yp.Type != "" {
		types = append(types, yp.Type)
	} else {
		types = append(types, yp.Types...)
	}

	var opts []*TypeOption

	for _, t := range types {
		ts, err := parseType(ytypes, t)
		if err != nil {
			return nil, err
		}
		opts = append(opts, ts...)
	}

	if len(opts) == 1 {
		o := opts[0]
		if o.Choices == nil {
			o.Choices = append(o.Choices, yp.Choices...)
		}
		if o.Description == "" {
			o.Description = yp.Description
		}
		if o.Type == "object" {
			if len(o.Sections) == 0 {
				sections, err := parseSections(ytypes, yp.Sections)
				if err != nil {
					return nil, err
				}
				o.Sections = sections
			}
		}
	}

	// Assume properties are reloadable by default.
	reloadable := true
	if yp.Reloadable != nil {
		reloadable = *yp.Reloadable
	}

	p := Property{
		Name:           strings.TrimSpace(yp.Name),
		Description:    strings.TrimSpace(yp.Description),
		Types:          opts,
		Disabled:       yp.Disabled,
		Default:        yp.Default,
		Deprecation:    strings.TrimSpace(yp.Deprecation),
		Examples:       yp.Examples,
		Aliases:        yp.Aliases,
		Reloadable:     reloadable,
		ReloadableNote: strings.TrimSpace(yp.ReloadableNote),
		URL:            yp.URL,
	}

	return &p, nil
}

func parseType(ytypes map[string]*yamlType, t string) ([]*TypeOption, error) {
	var (
		isArray bool
		isMap   bool
	)

	if m := arrayTypeRe.FindStringSubmatch(t); len(m) == 2 {
		isArray = true
		t = m[1]
	}
	if m := mapTypeRe.FindStringSubmatch(t); len(m) == 2 {
		isMap = true
		t = m[1]
	}

	// Primitive types.
	if _, ok := primitiveTypes[t]; ok {
		return []*TypeOption{{
			Type:  t,
			Map:   isMap,
			Array: isArray,
		}}, nil
	}

	// Dereference non-primitive types.
	b, ok := ytypes[t]
	if !ok {
		return nil, fmt.Errorf("unknown type %q", t)
	}

	bp, err := parseProperty(ytypes, b)
	if err != nil {
		return nil, err
	}

	var tos []*TypeOption
	for _, t := range bp.Types {
		x := &TypeOption{
			Description: t.Description,
			Type:        t.Type,
			Sections:    t.Sections,
			Choices:     t.Choices,
		}

		// Wrap with parent type.
		if isMap {
			if t.Map {
				x.MapOfMap = true
			} else if t.Array {
				x.MapOfArray = true
			} else {
				x.Map = true
			}
		} else if isArray {
			if t.Map {
				x.ArrayOfMap = true
			} else if t.Array {
				x.ArrayOfArray = true
			} else {
				x.Array = true
			}
		}
		tos = append(tos, x)
	}

	return tos, nil
}
