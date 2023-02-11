// Command generate_test generates a C++ source file implementing a unit test
// for Advent of Code solutions.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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
	part1Pairs    = flag.String("part1_pairs", "", `Comma-separated list of file pairs of the form "name:in_file:out_file" containing problem inputs and corresponding expected outputs.`)
	part2Pairs    = flag.String("part2_pairs", "", `Comma-separated list of file pairs of the form "name:in_file:out_file" containing problem inputs and corresponding expected outputs.`)

	output = flag.String("output", "", "Path to write the generated file to. Leaving this empty writes the file to stdout.")
)

var (
	//go:embed dayXX_test.cc.template
	rawTemplate string
	tmpl        = template.Must(template.New("dayXX_test.cc").Parse(rawTemplate))
)

type inOutPair struct {
	Name    string
	In, Out string
}

type templateArgs struct {
	HeaderFile             string
	Namespace              string
	Part1Func, Part2Func   string
	Part1Pairs, Part2Pairs []inOutPair
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

	if *part1Pairs == "" {
		return errors.New("-part1_pairs is required but was empty")
	}
	for _, pair := range strings.Split(*part1Pairs, ",") {
		parts := strings.Split(pair, ":")
		if len(parts) != 3 {
			return fmt.Errorf("-part1_pairs contains invalid element %q", pair)
		}
		name := parts[0]
		in, out := parts[1], parts[2]
		inData, err := os.ReadFile(in)
		if err != nil {
			return fmt.Errorf("-part1_pairs contained unreadable file: %v", err)
		}
		outData, err := os.ReadFile(out)
		if err != nil {
			return fmt.Errorf("-part1_pairs contained unreadable file: %v", err)
		}
		args.Part1Pairs = append(args.Part1Pairs, inOutPair{
			Name: name,
			In:   string(inData),
			Out:  string(outData),
		})
	}

	if *part2Pairs == "" {
		return errors.New("-part2_pairs is required but was empty")
	}
	for _, pair := range strings.Split(*part2Pairs, ",") {
		parts := strings.Split(pair, ":")
		if len(parts) != 3 {
			return fmt.Errorf("-part2_pairs contains invalid element %q", pair)
		}
		name := parts[0]
		in, out := parts[1], parts[2]
		inData, err := os.ReadFile(in)
		if err != nil {
			return fmt.Errorf("-part2_pairs contained unreadable file: %v", err)
		}
		outData, err := os.ReadFile(out)
		if err != nil {
			return fmt.Errorf("-part2_pairs contained unreadable file: %v", err)
		}
		args.Part2Pairs = append(args.Part2Pairs, inOutPair{
			Name: name,
			In:   string(inData),
			Out:  string(outData),
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
