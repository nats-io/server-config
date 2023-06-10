package config

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type hierPath struct {
	Name string
	Path string
}

func generateTemplate(w io.Writer, p *Property, dir, base string, hier []*hierPath, relative bool, indexName string, breadcrumbs bool) error {
	o := func(str string, args ...any) {
		fmt.Fprintf(w, str, args...)
	}

	o("# %s\n\n", p.Name)

	if breadcrumbs {
		for _, tok := range hier {
			var p string
			if relative {
				rp, err := filepath.Rel(base, tok.Path)
				if err != nil {
					return err
				}
				p = rp
			} else {
				p = tok.Path
			}

			if indexName != "" {
				p = filepath.Join(p, indexName)
			}

			l := fmt.Sprintf("[%s](%s)", tok.Name, p)
			o("/ %s ", l)
		}
		o("\n\n")
	}

	if p.Deprecation != "" {
		o("_**Deprecation notice.** %s_\n\n", p.Deprecation)
	}

	if p.Description != "" {
		o("%s\n\n", p.Description)
	}

	if p.Default != nil {
		o("*Default value*: `%v`\n\n", p.Default)
	}
	if p.Disabled {
		o("*Disabled by default*\n\n")
	}
	if len(p.Aliases) > 0 {
		o("*Aliases*\n\n")
		for _, a := range p.Aliases {
			o("- `%s`\n", a)
		}
		o("\n\n")
	}

	o("*Reloadable*: `%v`\n\n", p.Reloadable)

	if p.URL != "" {
		o("*URL*: `%s`\n\n", p.URL)
	}

	o("*Types*\n\n")
	for _, t := range p.Types {
		o("- `%s`\n", t)
	}
	o("\n\n")

	if len(p.Sections) > 0 {
		o("## Properties\n\n")

		for _, s := range p.Sections {
			if s.Name != "" {
				o("### %s\n\n", s.Name)
			}

			if s.Description != "" {
				o("%s\n\n", s.Description)
			}

			for _, x := range s.Properties {
				var path string
				if relative {
					path = x.Name
				} else {
					path = filepath.Join(base, x.Name)
				}
				if indexName != "" {
					path = filepath.Join(path, indexName)
				}
				o("#### [`%s`](%s)\n\n", x.Name, path)
				o("%s\n\n", x.Description)
				if x.Default != nil {
					o("Default value: `%v`\n\n", x.Default)
				}
				if x.Disabled {
					o("*Disabled by default*`\n\n")
				}
			}
		}
	}

	if len(p.Examples) > 0 {
		o("## Examples\n\n")

		for _, e := range p.Examples {
			if e.Label != "" {
				o("### %s\n", e.Label)
			}
			o("```\n")
			o("%v\n", e.Value)
			o("```\n")
		}
		o("\n")
	}

	return nil
}

// GenerateMarkdown generates a directory of markdown files, including
// the top-level and one for each nested property.
func GenerateMarkdown(config *Config, dir string, base string, relative bool, indexName string, breadcrumbs bool) error {
	buf := bytes.NewBuffer(nil)

	prop := Property{
		Name:        config.Name,
		Description: config.Description,
		Sections:    config.Sections,
	}

	if base == "/" {
		base = ""
	} else if strings.HasSuffix(base, "/") {
		base = base[:len(base)-1]
	}

	return generatePropMarkdown(&prop, buf, dir, base, nil, relative, indexName, breadcrumbs)
}

func generatePropMarkdown(prop *Property, buf *bytes.Buffer, dir, base string, hier []*hierPath, relative bool, indexName string, breadcrumbs bool) error {
	buf.Reset()

	fmt.Printf("%s- [%s](%s)\n", strings.Repeat("  ", len(hier)), prop.Name, filepath.Join(base, indexName))

	if err := generateTemplate(buf, prop, dir, base, hier, relative, indexName, breadcrumbs); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("make dir: %w", err)
	}

	path := filepath.Join(dir, indexName)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	var nhier []*hierPath
	nhier = append(nhier, hier...)
	nhier = append(nhier, &hierPath{Name: prop.Name, Path: base})

	for _, s := range prop.Sections {
		for _, p := range s.Properties {
			// Property gets its own directory.
			ndir := filepath.Join(dir, p.Name)
			nbase := filepath.Join(base, p.Name)
			if err := generatePropMarkdown(p, buf, ndir, nbase, nhier, relative, indexName, breadcrumbs); err != nil {
				return err
			}
		}
	}

	return nil
}
