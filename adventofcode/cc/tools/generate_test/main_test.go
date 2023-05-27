package main_test

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"go.saser.se/runfiles"
)

var (
	binary  = runfiles.MustPath("adventofcode/cc/tools/generate_test/generate_test_/generate_test")
	inFile  = runfiles.MustPath("adventofcode/cc/tools/generate_test/testdata/test.in")
	outFile = runfiles.MustPath("adventofcode/cc/tools/generate_test/testdata/test.out")
)

func TestGenerateTest(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags map[string]string
	}{
		{
			name: "OnlyHeader",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
			},
		},
		{
			name: "Part1ExplicitlyEmpty",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "",
				"-part1_pairs": "",
			},
		},
		{
			name: "Part2ExplicitlyEmpty",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part2_func":  "",
				"-part2_pairs": "",
			},
		},
		{
			name: "BothPartsExplicitlyEmpty",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "",
				"-part1_pairs": "",
				"-part2_func":  "",
				"-part2_pairs": "",
			},
		},
		{
			name: "OnlyPart1",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "Part1",
				"-part1_pairs": "test:" + inFile + ":" + outFile,
			},
		},
		{
			name: "OnlyPart1_ExplicitlyEmptyPart2",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "Part1",
				"-part1_pairs": "test:" + inFile + ":" + outFile,
				"-part2_func":  "",
				"-part2_pairs": "",
			},
		},
		{
			name: "OnlyPart2",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part2_func":  "Part2",
				"-part2_pairs": "test:" + inFile + ":" + outFile,
			},
		},
		{
			name: "OnlyPart2_ExplicitlyEmptyPart1",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "",
				"-part1_pairs": "",
				"-part2_func":  "Part2",
				"-part2_pairs": "test:" + inFile + ":" + outFile,
			},
		},
		{
			name: "BothPart1AndPart2",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "Part1",
				"-part1_pairs": "test:" + inFile + ":" + outFile,
				"-part2_func":  "Part2",
				"-part2_pairs": "test:" + inFile + ":" + outFile,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var args []string
			for flag, value := range tt.flags {
				args = append(args, flag+"="+value)
			}
			cmd := exec.Command(binary, args...)
			var stderr, stdout strings.Builder
			cmd.Stderr = &stderr
			cmd.Stdout = &stdout
			err := cmd.Run()
			if err != nil {
				lines := []string{
					fmt.Sprintf("Error: %v", err),
					fmt.Sprintf("Args: %q", args),
					fmt.Sprintf("stderr:\n%s", stderr.String()),
					fmt.Sprintf("stdout:\n%s", stdout.String()),
				}
				t.Error(strings.Join(lines, "\n"))
			} else {
				if stdout.Len() == 0 {
					t.Errorf("Program exited successfully but there was no output. Args:\n%q", args)
				}
			}
		})
	}
}

func TestGenerateTest_Error(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags map[string]string
	}{
		{
			name: "NoHeaderFile",
			flags: map[string]string{
				"-header_file": "",
				"-part1_func":  "Part1",
				"-part1_pairs": "test:" + inFile + ":" + outFile,
			},
		},
		{
			name: "Part1FuncButNoPairs",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "Part1",
				"-part1_pairs": "",
			},
		},
		{
			name: "Part1PairsButNoFunc",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "",
				"-part1_pairs": "test:" + inFile + ":" + outFile,
			},
		},
		{
			name: "Part2FuncButNoPairs",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part2_func":  "Part2",
				"-part2_pairs": "",
			},
		},
		{
			name: "Part2PairsButNoFunc",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part2_func":  "",
				"-part2_pairs": "test:" + inFile + ":" + outFile,
			},
		},
		{
			name: "PairWithoutName",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "Part1",
				"-part1_pairs": "" + inFile + ":" + outFile,
			},
		},
		{
			name: "PairWithEmptyName",
			flags: map[string]string{
				"-header_file": "adventofcode/cc/year2050/day01.h",
				"-part1_func":  "Part1",
				"-part1_pairs": ":" + inFile + ":" + outFile,
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var args []string
			for flag, value := range tt.flags {
				args = append(args, flag+"="+value)
			}
			cmd := exec.Command(binary, args...)
			var stderr, stdout strings.Builder
			cmd.Stderr = &stderr
			cmd.Stdout = &stdout
			err := cmd.Run()
			if err == nil { // if NO error
				lines := []string{
					"`generate_test` unexpectedly ran successfully.",
					fmt.Sprintf("Args: %q", args),
					fmt.Sprintf("stderr:\n%s", stderr.String()),
					fmt.Sprintf("stdout:\n%s", stdout.String()),
				}
				t.Error(strings.Join(lines, "\n"))
			}
		})
	}
}
