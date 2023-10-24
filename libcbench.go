// libcbench compares llvm libc benchmark results using benchstat.
//
// For details of libc benchmarking see:
// https://github.com/llvm/llvm-project/tree/main/libc/benchmarks
//
// Usage:
//
//	go install golang.org/x/perf/cmd/benchstat@latest
//	go install github.com/dvyukov/libcbench@latest
//
//	libcbench [benchstat flags] baseline.json experiment.json
//
//	                 │  baseline   │              experiment            │
//	                 │   sec/op    │   sec/op     vs base               │
//	memmove/Google_A   3.910n ± 0%   3.885n ± 0%  -0.66% (p=0.008 n=50)
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Study struct {
	StudyName     string
	Configuration struct {
		Function             string
		IsSweepMode          bool
		NumTrials            int
		SizeDistributionName string
	}
	Measurements []float64
}

func main() {
	var flags []string
	var files []string
	for _, arg := range os.Args[1:] {
		if arg != "" && arg[0] == '-' {
			flags = append(flags, arg)
		} else {
			files = append(files, arg)
		}
	}
	if err := process(flags, files); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func process(flags, files []string) error {
	cmd := exec.Command("benchstat", flags...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	for fileIdx, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		study := new(Study)
		if err := json.Unmarshal(data, study); err != nil {
			return fmt.Errorf("failed to parse %v: %w", file, err)
		}
		name := study.Configuration.Function
		if pos := strings.LastIndexByte(name, '.'); pos != -1 {
			name = name[pos+1:]
		}
		typ := strings.Replace(strings.TrimSpace(strings.TrimPrefix(
			study.Configuration.SizeDistributionName, name)), " ", "_", -1)
		pr, pw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		cmd.ExtraFiles = append(cmd.ExtraFiles, pr)
		cmd.Args = append(cmd.Args, fmt.Sprintf("%v=/dev/fd/%v", study.StudyName, fileIdx+3))
		go func() {
			size := 0
			for i, v := range study.Measurements {
				if study.Configuration.IsSweepMode {
					if i%study.Configuration.NumTrials == 0 {
						size++
					}
					typ = fmt.Sprintf("%v", size)
				}
				fmt.Fprintf(pw, "Benchmark%v/%v 1 %v ns/op\n", name, typ, v*1e9)
			}
			pw.Close()
		}()
	}
	return cmd.Run()
}
