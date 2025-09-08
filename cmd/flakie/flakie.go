package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type TestResult struct {
	Package string `json:"package"`
	Test    string `json:"test"`
	Run     int    `json:"run"`
	Passed  bool   `json:"passed"`
	Output  string `json:"output"`
}

type FlakySummary struct {
	TotalRuns   int                       `json:"total_runs"`
	Packages    []string                  `json:"packages"`
	TestStats   map[string]map[string]int `json:"test_stats"` // testName -> {pass: x, fail: y}
	FlakyTests  []string                  `json:"flaky_tests"`
	FailedTests []string                  `json:"failed_tests"`
}

func main() {
	var (
		runs       int
		pkg        string
		timeoutStr string
		jsonOut    bool
		race       bool
		extraArgs  string
	)

	flag.IntVar(&runs, "runs", 5, "Number of times to run the test suite")
	flag.StringVar(&pkg, "pkg", "./...", "Package pattern to test (e.g., ./... or ./pkg/...")
	flag.StringVar(&timeoutStr, "timeout", "10m", "Timeout for each test run (e.g., 5m, 30s)")
	flag.BoolVar(&jsonOut, "json", false, "Print JSON summary in addition to human-readable output")
	flag.BoolVar(&race, "race", false, "Enable the race detector")
	flag.StringVar(&extraArgs, "args", "", "Extra args to pass to 'go test' (quoted)")
	flag.Parse()

	timeout, err := time.ParseDuration(timeoutStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid timeout: %v\n", err)
		os.Exit(2)
	}

	reRun := regexp.MustCompile(`^=== RUN\s+([A-Za-z0-9_\-/]+)`)                                      // === RUN   TestXxx
	rePass := regexp.MustCompile(`^--- PASS: \s*([A-Za-z0-9_\-/]+)`)                                  // --- PASS: TestXxx
	reFail := regexp.MustCompile(`^--- FAIL: \s*([A-Za-z0-9_\-/]+)`)                                  // --- FAIL: TestXxx
	rePkg := regexp.MustCompile(`^\?\s+([^\s]+)\s+\[no test files\]|^ok\s+([^\s]+)|^FAIL\s+([^\s]+)`) // capture package

	// stats[testName] = map["pass"]count, map["fail"]count
	stats := map[string]map[string]int{}
	packagesSet := map[string]struct{}{}
	var flakyFound bool

	for i := 1; i <= runs; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		args := []string{"test", pkg, "-count=1", "-run", ".", "-v"}
		if race {
			args = append(args, "-race")
		}
		if strings.TrimSpace(extraArgs) != "" {
			// naive split on spaces; users can pass quoted args as a single string
			args = append(args, strings.Fields(extraArgs)...)
		}

		cmd := exec.CommandContext(ctx, "go", args...)
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf
		err := cmd.Run()

		// parse output
		scanner := bufio.NewScanner(bytes.NewReader(outBuf.Bytes()))
		scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
		currentPkg := ""
		seenInRun := map[string]bool{}
		for scanner.Scan() {
			line := scanner.Text()
			if m := rePkg.FindStringSubmatch(line); m != nil {
				for _, g := range m[1:] {
					if g != "" {
						currentPkg = g
						packagesSet[currentPkg] = struct{}{}
						break
					}
				}
			}
			if m := reRun.FindStringSubmatch(line); m != nil {
				// ensure presence
				t := m[1]
				if _, ok := stats[t]; !ok {
					stats[t] = map[string]int{"pass": 0, "fail": 0}
				}
				seenInRun[t] = true
				continue
			}
			if m := rePass.FindStringSubmatch(line); m != nil {
				t := m[1]
				if _, ok := stats[t]; !ok {
					stats[t] = map[string]int{"pass": 0, "fail": 0}
				}
				stats[t]["pass"]++
				continue
			}
			if m := reFail.FindStringSubmatch(line); m != nil {
				t := m[1]
				if _, ok := stats[t]; !ok {
					stats[t] = map[string]int{"pass": 0, "fail": 0}
				}
				stats[t]["fail"]++
				continue
			}
		}
		if scanErr := scanner.Err(); scanErr != nil {
			fmt.Fprintf(os.Stderr, "error scanning test output: %v\n", scanErr)
		}

		// When 'go test' exits non-zero, err != nil; that's fine; we already counted failures
		_ = err
	}

	// compute summary
	var packages []string
	for p := range packagesSet {
		packages = append(packages, p)
	}

	var flaky []string
	var failed []string
	for t, c := range stats {
		if c["pass"] > 0 && c["fail"] > 0 {
			flaky = append(flaky, t)
			flakyFound = true
		} else if c["pass"] == 0 && c["fail"] > 0 {
			failed = append(failed, t)
		}
	}

	summary := FlakySummary{
		TotalRuns:   runs,
		Packages:    packages,
		TestStats:   stats,
		FlakyTests:  flaky,
		FailedTests: failed,
	}

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(summary)
	} else {
		printHuman(summary)
	}

	if flakyFound {
		os.Exit(3)
	}
	if len(failed) > 0 {
		os.Exit(1)
	}
}

func printHuman(s FlakySummary) {
	if len(s.FlakyTests) == 0 && len(s.FailedTests) == 0 {
		fmt.Printf("No flaky tests detected after %d runs.\n", s.TotalRuns)
		return
	}

	if len(s.FlakyTests) > 0 {
		fmt.Println("Flaky tests detected:")
		for _, t := range s.FlakyTests {
			c := s.TestStats[t]
			fmt.Printf("- %s (pass=%d, fail=%d)\n", t, c["pass"], c["fail"])
		}
	}
	if len(s.FailedTests) > 0 {
		fmt.Println("Consistently failing tests:")
		for _, t := range s.FailedTests {
			c := s.TestStats[t]
			fmt.Printf("- %s (fail=%d)\n", t, c["fail"])
		}
	}
}
