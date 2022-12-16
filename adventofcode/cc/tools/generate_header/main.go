package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"text/template"

	"k8s.io/klog/v2"

	// For statically embedding the template.
	_ "embed"
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

var (
	year  = flag.Int("year", 0, "The year. Must be 2015 or later.")
	day   = flag.Int("day", 0, "The day. Must be in the range 1-25.")
	part1 = flag.Bool("part1", true, "Whether to generate a function for part 1.")
	part2 = flag.Bool("part2", true, "Whether to generate a function for part 2.")

	output = flag.String("output", "", "Path to write the generated file to. Leaving this empty writes the file to stdout.")
)

var (
	//go:embed dayXX.h.template
	rawTemplate string
	tmpl        = template.Must(template.New("dayXX.h.template").Parse(rawTemplate))
)

type templateArgs struct {
	Year         int
	Day          int
	Part1, Part2 bool
}

func errmain() error {
	if *year < 2015 {
		return fmt.Errorf("-year=%d must be 2015 or later", *year)
	}
	if *day < 1 || *day > 25 {
		return fmt.Errorf("-day=%d must be in the range 1-25", *day)
	}
	if *day == 25 && *part2 {
		return errors.New("-day=25 and -part2=true, but there's (generally) not a part 2 for day 25")
	}

	args := templateArgs{
		Year:  *year,
		Day:   *day,
		Part1: *part1,
		Part2: *part2,
	}
	var w io.Writer
	if *output == "" {
		w = os.Stdout
	} else {
		f, err := os.Create(*output)
		if err != nil {
			return fmt.Errorf("create output file: %v", err)
		}
		defer f.Close()
		w = f
	}
	if err := tmpl.Execute(w, args); err != nil {
		return fmt.Errorf("render template: %v", err)
	}
	return nil
}

func main() {
	flag.Parse()
	if err := errmain(); err != nil {
		klog.Exit(err)
	}
}
