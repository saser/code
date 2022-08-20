package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"text/template"

	"github.com/golang/glog"

	// For embedding the template.
	_ "embed"
)

var (
	region = flag.String("region", "", "GCP region.")
	output = flag.String("output", "", "Path to write the generated file to.")
)

var (
	//go:embed config.json.template
	rawConfigTemplate string
	configTemplate    = template.Must(template.New("config.json").Parse(string(rawConfigTemplate)))
)

type configTemplateParameters struct {
	Region string
}

func errmain() error {
	flag.Parse()

	if *region == "" {
		return errors.New("-region is empty")
	}
	if *output == "" {
		return errors.New("-output is empty")
	}

	params := configTemplateParameters{
		Region: *region,
	}
	var buf bytes.Buffer
	if err := configTemplate.Execute(&buf, params); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.WriteFile(*output, buf.Bytes(), fs.FileMode(0o644)); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	return nil
}

func main() {
	if err := errmain(); err != nil {
		glog.Exit(err)
	}
}
