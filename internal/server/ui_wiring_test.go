package server

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestTemplateEventHandlersReferenceImplementedAppMethods(t *testing.T) {
	root := repoRoot(t)
	appJS := mustReadFile(t, filepath.Join(root, "web/static/app.js"))
	implemented := appMethodNames(appJS)

	templatePaths := []string{
		"web/templates/index.html",
		"web/templates/battlemaps.html",
		"web/templates/armies.html",
	}

	var missing []string
	for _, templatePath := range templatePaths {
		body := mustReadFile(t, filepath.Join(root, templatePath))
		for _, expression := range alpineEventHandlerExpressions(body) {
			for _, method := range calledMethods(expression) {
				if !implemented[method] && !knownBrowserGlobal(method) {
					missing = append(missing, templatePath+": "+method+" in "+expression)
				}
			}
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("template event handlers call methods missing from app.js:\n%s", strings.Join(missing, "\n"))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root containing go.mod")
		}
		dir = parent
	}
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func appMethodNames(appJS string) map[string]bool {
	methods := map[string]bool{}
	methodPattern := regexp.MustCompile(`(?m)^\s*(?:async\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\s*\([^)]*\)\s*\{`)
	for _, match := range methodPattern.FindAllStringSubmatch(appJS, -1) {
		methods[match[1]] = true
	}
	return methods
}

func alpineEventHandlerExpressions(template string) []string {
	attributePattern := regexp.MustCompile(`(?:@|x-on:)(?:click|change|input|wheel)(?:\.[A-Za-z0-9_-]+)*="([^"]+)"`)
	matches := attributePattern.FindAllStringSubmatch(template, -1)
	expressions := make([]string, 0, len(matches))
	for _, match := range matches {
		expressions = append(expressions, match[1])
	}
	return expressions
}

func calledMethods(expression string) []string {
	callPattern := regexp.MustCompile(`\b([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)
	matches := callPattern.FindAllStringSubmatch(expression, -1)
	methods := make([]string, 0, len(matches))
	for _, match := range matches {
		methods = append(methods, match[1])
	}
	return methods
}

func knownBrowserGlobal(name string) bool {
	switch name {
	case "Boolean", "Math", "Number", "String":
		return true
	default:
		return false
	}
}
