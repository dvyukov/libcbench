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
	results, err := parseFiles(files)
	if err != nil {
		return err
	}
	cmd := exec.Command("benchstat", flags...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	for idx, res := range results {
		idx, res := idx, res
		pr, pw, err := os.Pipe()
		if err != nil {
			return fmt.Errorf("failed to create pipe: %w", err)
		}
		cmd.ExtraFiles = append(cmd.ExtraFiles, pr)
		cmd.Args = append(cmd.Args, fmt.Sprintf("%v=/dev/fd/%v", res.Name, idx+3))
		go func() {
			for _, bench := range res.Benchmarks {
				fmt.Fprintf(pw, "Benchmark%v 1 %v ns/op\n", bench.Name, bench.Val)
			}
			pw.Close()
		}()
	}
	return cmd.Run()
}

type Result struct {
	Name       string
	Benchmarks []Benchmark
}

type Benchmark struct {
	Name string
	Val  float64
}

func parseFiles(files []string) ([]*Result, error) {
	var results []*Result
	names := make(map[string]*Result)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		study := new(Study)
		if err := json.Unmarshal(data, study); err != nil {
			return nil, fmt.Errorf("failed to parse %v: %w", file, err)
		}
		res := names[study.StudyName]
		if res == nil {
			res = &Result{
				Name: study.StudyName,
			}
			names[study.StudyName] = res
			results = append(results, res)
		}
		name := study.Configuration.Function
		if pos := strings.LastIndexByte(name, '.'); pos != -1 {
			name = name[pos+1:]
		}
		typ := strings.Replace(strings.TrimSpace(strings.TrimPrefix(
			study.Configuration.SizeDistributionName, name)), " ", "_", -1)
		size := 0
		for i, v := range study.Measurements {
			if study.Configuration.IsSweepMode {
				if i%study.Configuration.NumTrials == 0 {
					size++
				}
				typ = fmt.Sprintf("%v", size)
			}
			res.Benchmarks = append(res.Benchmarks, Benchmark{
				Name: name + "/" + typ,
				Val:  v * 1e9,
			})
		}
	}
	return results, nil
}
