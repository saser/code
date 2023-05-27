package main_test

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"go.saser.se/runfiles"
)

var binary = runfiles.MustPath("adventofcode/cc/tools/generate_header/generate_header_/generate_header")

func TestGenerateHeader(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags map[string]string
	}{
		{
			name: "NeitherPart",
			flags: map[string]string{
				"-year": "2023",
				"-day":  "10",
			},
		},
		{
			name: "Part1",
			flags: map[string]string{
				"-year":  "2023",
				"-day":   "10",
				"-part1": "true",
			},
		},
		{
			name: "Part2",
			flags: map[string]string{
				"-year":  "2023",
				"-day":   "10",
				"-part2": "true",
			},
		},
		{
			name: "BothParts",
			flags: map[string]string{
				"-year":  "2023",
				"-day":   "10",
				"-part1": "true",
				"-part2": "true",
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

func TestGenerateHeader_Error(t *testing.T) {
	for _, tt := range []struct {
		name  string
		flags map[string]string
	}{
		{
			name: "YearTooLow",
			flags: map[string]string{
				"-year": "1995",
				"-day":  "10",
			},
		},
		{
			name: "DayTooLow",
			flags: map[string]string{
				"-year": "2023",
				"-day":  "0",
			},
		},
		{
			name: "DayNegative",
			flags: map[string]string{
				"-year": "2023",
				"-day":  "-10",
			},
		},
		{
			name: "DayTooHigh",
			flags: map[string]string{
				"-year": "2023",
				"-day":  "26",
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
					"`generate_header` unexpectedly ran successfully.",
					fmt.Sprintf("Args: %q", args),
					fmt.Sprintf("stderr:\n%s", stderr.String()),
					fmt.Sprintf("stdout:\n%s", stdout.String()),
				}
				t.Error(strings.Join(lines, "\n"))
			}
		})
	}
}
