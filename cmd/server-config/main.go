package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	config "github.com/nats-io/server-config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var (
		configYaml    string
		typesDir      string
		dirName       string
		basePath      string
		useRelative   bool
		indexFilename string
		trimIndex     bool
		breadcrumbs   bool
		renderTable   bool
	)

	flag.StringVar(&configYaml, "config", "config.yaml", "The root config YAML file.")
	flag.StringVar(&typesDir, "types", "types", "The path to the types directory.")
	flag.StringVar(&dirName, "dir", "ref", "The output directory for the reference docs.")
	flag.StringVar(&basePath, "base", "", "Base URL path for the ref document paths.")
	flag.BoolVar(&useRelative, "relative", false, "Use relative paths for the links.")
	flag.StringVar(&indexFilename, "indexname", "index.md", "The index filename for a directory.")
	flag.BoolVar(&trimIndex, "trimindex", false, "Trim the index filename from the URL path.")
	flag.BoolVar(&breadcrumbs, "breadcrumbs", false, "Include breadcrumbs navigation to a page.")
	flag.BoolVar(&renderTable, "table", false, "Render properties as tables.")

	flag.Parse()

	var paths []string
	entries, err := os.ReadDir(typesDir)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}
	for _, e := range entries {
		paths = append(paths, filepath.Join(typesDir, e.Name()))
	}

	c, err := config.Parse(configYaml, paths)
	if err != nil {
		return err
	}

	mc := config.MarkdownConfig{
		BasePath:      basePath,
		RelativeLinks: useRelative,
		IndexName:     indexFilename,
		TrimIndexFile: trimIndex,
		Breadcrumbs:   breadcrumbs,
		TableProps:    renderTable,
	}

	//config.GenerateConfig(os.Stdout, c)
	return config.GenerateMarkdown(c, dirName, &mc)
}
