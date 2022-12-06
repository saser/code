// Command generate_test generates a C++ source file implementing a unit test
// for Advent of Code solutions.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"k8s.io/klog/v2"

	// For statically embedding the template.
	_ "embed"
)

func init() {
	klog.InitFlags(flag.CommandLine)
}

var (
	headerFile    = flag.String("header_file", "", `The main header file in which the solver functions are declared. Should be in the format it will be included, e.g., "adventofcode/cc/year2022/day01.h".`)
	namespaceFlag = flag.String("namespace", "", `The namespace in which the solver functions live. Should be double-colon-separated, e.g., "adventofcode::cc::year2022::day01". If left empty the value will be derived from the -header_file flag by replacing slashes with double colons and stripping the .h suffix, e.g., "adventofcode/cc/year2022/day01.h" => "adventofcode::cc::year2022::day01".`)
	part1Func     = flag.String("part1_func", "Part1", "The name of the function solving part 1.")
	part2Func     = flag.String("part2_func", "Part2", "The name of the function solving part 2.")
	inputs        = flag.String("inputs", "", `Comma-separated list of file containing problem inputs.`)

	output = flag.String("output", "", "Path to write the generated file to. Leaving this empty writes the file to stdout.")
)

var (
	//go:embed dayXX_benchmark.cc.template
	rawTemplate string
	tmpl        = template.Must(template.New("dayXX_benchmark.cc").Parse(rawTemplate))
)

type inputPair struct {
	File, Input string
}

type templateArgs struct {
	HeaderFile           string
	Namespace            string
	Part1Func, Part2Func string
	Inputs               []inputPair
}

func errmain() error {
	var args templateArgs

	if *headerFile == "" {
		return errors.New("-header_file is required but was empty")
	}
	args.HeaderFile = *headerFile

	namespace := *namespaceFlag
	if namespace == "" {
		namespace = strings.ReplaceAll(strings.TrimSuffix(*headerFile, ".h"), "/", "::")
	}
	args.Namespace = namespace

	if *part1Func == "" {
		return errors.New("-part1_func is required but was empty")
	}
	args.Part1Func = *part1Func

	if *part2Func == "" {
		return errors.New("-part2_func is required but was empty")
	}
	args.Part2Func = *part2Func

	if *inputs == "" {
		return errors.New("-inputs is required but was empty")
	}
	for _, file := range strings.Split(*inputs, ",") {
		input, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("-inputs contained unreadable file: %v", err)
		}
		args.Inputs = append(args.Inputs, inputPair{
			File:  filepath.Base(file),
			Input: string(input),
		})
	}

	klog.V(1).Infof("Template args: %+v", args)

	var out io.Writer
	if *output == "" {
		out = os.Stdout
	} else {
		f, err := os.Create(*output)
		if err != nil {
			return fmt.Errorf("create output file: %v", err)
		}
		defer f.Close()
		out = f
	}
	if err := tmpl.Execute(out, args); err != nil {
		return fmt.Errorf("execute template: %v", err)
	}
	return nil
}

func main() {
	flag.Parse()
	if err := errmain(); err != nil {
		klog.Exit(err)
	}
}
