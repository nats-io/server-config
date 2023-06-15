package config

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config models the configuration.
type Config struct {
	// Name used for doc generation.
	Name string
	// Top-level config description for doc generation.
	Description string
	// Sections are the top-level sections for the config. This is only
	// used for logical ordering for the docs. It has no applicability
	// to the config itself.
	Sections []*Section
	// Types is an index of custom-defined types, e.g. `tls`, `listen`,
	// `user`, etc.
	Types map[string]*Property
}

// Section provides logical naming and organization for properties.
type Section struct {
	// Name of the section, e.g. "Connectivity"
	Name string
	// URL is an optional URL to a page with more information this section.
	URL string
	// Description of the section.
	Description string
	// Properties contains the ordered set of properties within this section.
	Properties []*Property
}

// Property models a configuration property.
type Property struct {
	// Name of the property, e.g. `host` or `jetstream`.
	Name string
	// Types are the set of types this property's value could be.
	Types []*TypeOption
	// URL is an optional URL to a page with more information about
	// this property.
	URL string
	// Description of the property.
	Description string
	// Deprecation is an optional note on the property being deprecated
	// and whether there is an alternate property to use.
	Deprecation string
	// Default value for this property. In practice, this only applies to
	// primitive values.
	Default any
	// Disabled is applied when generating a config file to explicitly
	// comment out a property. For example, when the `cluster` block is
	// present, it implies that it is enabled. If this property is true,
	// the generated config file will comment this block out.
	Disabled bool
	// Examples are a set of example values.
	Examples []*Example
	// Aliases are the set of aliases for this property, e.g. `subscribe`
	// and `sub`.
	Aliases []string
	// Reloadable indicates a change to this property in a server config can
	// be hot-reloaded rather than a hard restart of the server.
	Reloadable     bool
	ReloadableNote string
	// Sections nested under this property. This only applies to object-based
	// properties, e.g. `cluster {...}`.
	Sections []*Section
	// Version indicates the version of the server this property
	// became available.
	Version string

	Choices []string

	// Denotes the value is an array of other value types.
	Array bool
	// Denotes the value is an arbitrary map of string to any value type.
	Map bool

	typeRefs []string
}

type Example struct {
	// Short label for the example.
	Label string

	// Longer description of the example, noting specific details.
	Description string

	// The value, which will be rendered as code.
	Value string
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
		//Types:       types,
	}

	return &c, nil
}

type yamlFile struct {
	Types map[string]*yamlType
}

type yamlConfig struct {
	Name        string
	Description string
	Sections    []*yamlSection
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
	x := *p

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

	// TODO: deep copy needed?
	if len(x.Examples) == 0 {
		x.Examples = append(x.Examples, b.Examples...)
	}
	if len(x.Choices) == 0 {
		x.Choices = append(x.Choices, b.Choices...)
	}
	if len(x.Sections) == 0 {
		x.Sections = append(x.Sections, b.Sections...)
	}

	return &x
}

type yamlSection struct {
	Name        string
	Description string
	URL         string
	Properties  yaml.Node
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

var (
	arrayTypeRe = regexp.MustCompile(`^array\((.+)\)$`)
	mapTypeRe   = regexp.MustCompile(`^map\((.+)\)$`)
)

var (
	primitiveTypes = map[string]struct{}{
		"boolean":  {},
		"string":   {},
		"duration": {},
		"float":    {},
		"integer":  {},
		"object":   {},
	}
)

// TypeOption represents a value type for a property type.
// For example, the `jetstream` property can be a boolean, a string
// enum, or a JetStream object with its own set of properties.
type TypeOption struct {
	Description string
	// Defines the type, whether primitive or an object type.
	Type string
	// Denotes the value is an array of other value types.
	Array bool
	// Denotes the value is an arbitrary map of string to any value type.
	Map bool
	// For value types that are enums, this defines the set of choices.
	Choices []string

	Info *yamlType
}

func derefType(ytypes map[string]*yamlType, yp *yamlType, t string) ([]*TypeOption, error) {
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

	var vts []*TypeOption

	// Primitive types.
	if _, ok := primitiveTypes[t]; ok {
		var info *yamlType
		if t == "object" && len(yp.Sections) > 0 {
			info = yp
		}
		vts = append(vts, &TypeOption{
			Description: yp.Description,
			Type:        t,
			Map:         isMap,
			Array:       isArray,
			Choices:     yp.Choices,
			Info:        info,
		})

		return vts, nil
	}

	// Dereference non-primitive types.
	b, ok := ytypes[t]
	if !ok {
		return nil, fmt.Errorf("unknown type %q", t)
	}

	bvts, err := derefTypes(ytypes, b)
	if err != nil {
		return nil, err
	}

	vts = append(vts, bvts...)

	return vts, nil
}

func derefTypes(ytypes map[string]*yamlType, yp *yamlType) ([]*TypeOption, error) {
	// Normalize.
	var types []string
	if yp.Type != "" {
		types = append(types, yp.Type)
	} else {
		types = append(types, yp.Types...)
	}

	var vts []*TypeOption

	for _, t := range types {
		ts, err := derefType(ytypes, yp, t)
		if err != nil {
			return nil, err
		}

		vts = append(vts, ts...)
	}

	return vts, nil
}

// parseProperty recursively builds a property from the raw property info.
// The provided `type` or `types` dictates how the property is constructed.
// The simplest case is a single primitive type, e.g. `string`.
func parseProperty(ytypes map[string]*yamlType, yp *yamlType) (*Property, error) {
	dtypes, err := derefTypes(ytypes, yp)
	if err != nil {
		return nil, err
	}

	var nyp *TypeOption
	var otyps []*TypeOption

	for _, dt := range dtypes {
		if dt.Info != nil {
			if nyp != nil {
				return nil, fmt.Errorf("property %q has multiple info types", yp.Name)
			}
			nyp = dt
		} else {
			otyps = append(otyps, dt)
		}
	}

	// Hoist up the referenced type.
	if nyp != nil {
		yp = yp.Combine(nyp.Info)
		yp.Type = nyp.Type
	}

	// If this property has sections, recursively parse them.
	sections, err := parseSections(ytypes, yp.Sections)
	if err != nil {
		return nil, err
	}

	// Assume properties are reloadable by default.
	reloadable := true
	if yp.Reloadable != nil {
		reloadable = *yp.Reloadable
	}

	p := Property{
		Name:           strings.TrimSpace(yp.Name),
		Description:    strings.TrimSpace(yp.Description),
		Types:          otyps,
		Disabled:       yp.Disabled,
		Default:        yp.Default,
		Deprecation:    strings.TrimSpace(yp.Deprecation),
		Examples:       yp.Examples,
		Aliases:        yp.Aliases,
		Reloadable:     reloadable,
		ReloadableNote: strings.TrimSpace(yp.ReloadableNote),
		URL:            yp.URL,
		Choices:        yp.Choices,
		Sections:       sections,
	}

	return &p, nil
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
