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

func yesno(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func generateTemplate(w io.Writer, p *Property, mc *MarkdownConfig, hier []*hierPath) error {
	o := func(str string, args ...any) {
		fmt.Fprintf(w, str, args...)
	}

	o("# %s\n\n", p.Name)

	if mc.Breadcrumbs {
		for _, tok := range hier {
			var p string
			if mc.RelativeLinks {
				rp, err := filepath.Rel(mc.BasePath, tok.Path)
				if err != nil {
					return err
				}
				p = rp
			} else {
				p = tok.Path
			}

			if !mc.TrimIndexFile {
				p = filepath.Join(p, mc.IndexName)
			}

			l := fmt.Sprintf("[%s](%s)", tok.Name, p)
			o("/ %s ", l)
		}
		o("\n\n")
	}

	bpath := mc.BasePath
	if len(hier) > 0 {
		bpath = filepath.Join(hier[len(hier)-1].Path, p.Name)
	}

	if p.Deprecation != "" {
		o(`{%% callout type="warning" %%}
**Deprecation notice**
%s
{%% /callout %%}
`, p.Deprecation)
	}

	if p.Description != "" {
		o("%s\n\n", p.Description)
	}

	if p.Default != nil {
		o("- Default value: `%v`\n", p.Default)
	} else {
		o("- Default value: n/a\n")
	}
	if p.ReloadableNote != "" {
		o("- Hot reloadable: %s. %s\n", yesno(p.Reloadable), p.ReloadableNote)
	} else {
		o("- Hot reloadable: %s\n", yesno(p.Reloadable))
	}
	if p.Version != "" {
		o("- Version introduced: %s\n", p.Version)
	}
	if len(p.Aliases) > 0 {
		var aliases []string
		for _, a := range p.Aliases {
			aliases = append(aliases, fmt.Sprintf("`%s`", a))
		}
		o("- Aliases: %s\n", strings.Join(aliases, ", "))
	}

	o("\n")

	o("## Values\n\n")

	o("| Type | Description | Choices |\n")
	o("| :--- | :---------- | :------ |\n")

	for _, t := range p.Types {
		ft := t.Type
		if t.Array {
			ft = fmt.Sprintf("[ %s ]", ft)
		} else if t.Map {
			ft = fmt.Sprintf("{ string: %s }", ft)
		} else if t.ArrayOfMap {
			ft = fmt.Sprintf("[ { string: %s } ]", ft)
		} else if t.MapOfArray {
			ft = fmt.Sprintf("{ string: [ %s ] }", ft)
		} else if t.MapOfMap {
			ft = fmt.Sprintf("{ string: { string: %s } }", ft)
		} else if t.ArrayOfArray {
			ft = fmt.Sprintf("[ [ %s ] ]", ft)
		}

		var choices []string
		for _, c := range t.Choices {
			choices = append(choices, fmt.Sprintf("`%v`", c))
		}
		var choicesVal string
		if len(choices) > 0 {
			choicesVal = strings.Join(choices, ", ")
		} else {
			choicesVal = "-"
		}
		desc := "-"
		if t.Description != p.Description {
			desc = strings.ReplaceAll(t.Description, "\n", " ")
		}
		o("| %s | %s | %s |\n", ft, desc, choicesVal)

		if len(t.Sections) > 0 {
			o("## Properties\n\n")

			for _, s := range t.Sections {
				if s.Name != "" {
					o("### %s\n\n", s.Name)
				}

				if s.Description != "" {
					o("%s\n\n", s.Description)
				}

				o("| Name | Description | Default |\n")
				o("| :--- | :---------- | :------ |\n")

				for _, x := range s.Properties {
					var path string
					if mc.RelativeLinks {
						path = x.Name
					} else {
						path = filepath.Join(bpath, x.Name)
					}
					if !mc.TrimIndexFile {
						path = filepath.Join(path, mc.IndexName)
					}

					desc := strings.ReplaceAll(x.Description, "\n", " ")
					def := "-"
					if x.Default != nil {
						def = fmt.Sprintf("`%v`", x.Default)
					}
					o("| [%s](%s) | %s | %s |\n", x.Name, path, desc, def)
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
			if e.Description != "" {
				o("%s\n", strings.TrimSpace(e.Description))
			}
			o("```\n")
			o("%v\n", e.Value)
			o("```\n")
		}
		o("\n")
	}

	return nil
}

type MarkdownConfig struct {
	BasePath      string
	RelativeLinks bool
	IndexName     string
	TrimIndexFile bool
	Breadcrumbs   bool
}

// GenerateMarkdown generates a directory of markdown files, including
// the top-level and one for each nested property.
func GenerateMarkdown(config *Config, dir string, mc *MarkdownConfig) error {
	buf := bytes.NewBuffer(nil)

	prop := Property{
		Name:        config.Name,
		Description: config.Description,
		Types: []*TypeOption{
			{
				Type:     "object",
				Sections: config.Sections,
			},
		},
	}

	if mc.BasePath == "/" {
		mc.BasePath = ""
	} else if strings.HasSuffix(mc.BasePath, "/") {
		mc.BasePath = mc.BasePath[:len(mc.BasePath)-1]
	}

	return generatePropMarkdown(&prop, buf, dir, mc, nil)
}

func generatePropMarkdown(prop *Property, buf *bytes.Buffer, dir string, mc *MarkdownConfig, hier []*hierPath) error {
	buf.Reset()

	if err := generateTemplate(buf, prop, mc, hier); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("make dir: %w", err)
	}

	path := filepath.Join(dir, mc.IndexName)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	var nhier []*hierPath
	nhier = append(nhier, hier...)
	base := mc.BasePath
	if len(nhier) > 0 {
		base = filepath.Join(nhier[len(nhier)-1].Path, prop.Name)
	}
	nhier = append(nhier, &hierPath{
		Name: prop.Name,
		Path: base,
	})

	upath := strings.TrimPrefix(base, "/")
	if !mc.TrimIndexFile {
		upath = filepath.Join(upath, mc.IndexName)
	}
	fmt.Printf("%s* [%s](%s)\n", strings.Repeat("  ", len(hier)), prop.Name, upath)

	// Recurse into options having nested properties.
	for _, o := range prop.Types {
		if o.Type == "object" {
			for _, s := range o.Sections {
				for _, p := range s.Properties {
					// Property gets its own directory.
					ndir := filepath.Join(dir, p.Name)
					if err := generatePropMarkdown(p, buf, ndir, mc, nhier); err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}
