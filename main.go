package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"path"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/spudtrooper/goutil/check"
	"github.com/spudtrooper/goutil/io"
	"github.com/spudtrooper/goutil/sets"
)

var (
	dir    = flag.String("dir", ".", "Directory where go.mod to update is")
	depth  = flag.Int("depth", 1, "Number of levels up to search for other modules")
	dryRun = flag.Bool("dry_run", false, "Just print the new contents of go.mod")

	// module github.com/spudtrooper/uselocalrequires
	moduleRE = regexp.MustCompile(`^module\s+(\S+)$`)
	// require github.com/pkg/errors v0.9.1
	singleRequireRE = regexp.MustCompile(`^require (\S+) \S+$`)
	// require (
	requireStartRE = regexp.MustCompile(`^require \($`)
	// 		github.com/pkg/errors v0.9.1
	requireRE = regexp.MustCompile(`^\s+(\S+) \S+$`)
	// )
	requireEndRE = regexp.MustCompile(`^\)$`)
	// go 1.17
	goVersionRE = regexp.MustCompile(`^go [\d\.]+$`)
	// replace github.com/spudtrooper/goutil => ../goutil
	replaceRE = regexp.MustCompile(`^replace\s+(\S+)\s*=>\s*\S+$`)
)

type goModule struct {
	name   string // e.g. github.com/spudtrooper/uselocalrequires
	relDir string // relative path of the directory containing this module
}

func findRequires(goModFile string) ([]string, error) {
	var requires []string
	inRequires := false
	b, err := ioutil.ReadFile(goModFile)
	if err != nil {
		return nil, errors.Errorf("error reading %s: %v", goModFile, err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if inRequires {
			if requireEndRE.MatchString(line) {
				inRequires = false
				continue
			}
			if m := requireRE.FindStringSubmatch(line); len(m) == 2 {
				require := m[1]
				requires = append(requires, require)
				continue
			}
		} else {
			if requireStartRE.MatchString(line) {
				inRequires = true
				continue
			}
			if m := singleRequireRE.FindStringSubmatch(line); len(m) == 2 {
				require := m[1]
				requires = append(requires, require)
				continue
			}
		}
	}
	return requires, nil
}

func searchDir(dir string, goModules *[]goModule) error {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Errorf("error listing dir %s: %v", dir, err)
	}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		modDir := path.Join(dir, f.Name())
		goModFile := path.Join(modDir, "go.mod")
		if !io.FileExists(goModFile) {
			continue
		}
		b, err := ioutil.ReadFile(goModFile)
		if err != nil {
			return errors.Errorf("error reading %s: %v", goModFile, err)
		}
		for _, line := range strings.Split(string(b), "\n") {
			if m := moduleRE.FindStringSubmatch(line); len(m) == 2 {
				moduleName := m[1]
				goMod := goModule{
					name:   moduleName,
					relDir: modDir,
				}
				*goModules = append(*goModules, goMod)
				break
			}
		}
	}
	return nil
}

func findModules(dir string) ([]goModule, error) {
	var goModules []goModule
	for i := 0; i < *depth; i++ {
		dir := ".."
		for j := 0; j < i; j++ {
			dir = path.Join("..", dir)
		}
		if err := searchDir(dir, &goModules); err != nil {
			return nil, err
		}
	}
	return goModules, nil
}

type replacement struct {
	module string
	relDir string
}

func findNewContent(goModFile string, repls []replacement) (string, error) {
	b, err := ioutil.ReadFile(goModFile)
	if err != nil {
		return "", errors.Errorf("error reading %s: %v", goModFile, err)
	}

	var existingRepls []string
	for _, line := range strings.Split(string(b), "\n") {
		if m := replaceRE.FindStringSubmatch(line); len(m) == 2 {
			repl := m[1]
			existingRepls = append(existingRepls, repl)
		}
	}
	existingReplSet := sets.String(existingRepls)

	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		out = append(out, line)
		if goVersionRE.MatchString(line) {
			out = append(out, "")
			for _, r := range repls {
				if existingReplSet[r.module] {
					log.Printf("skipping existing module: %s", r.module)
					continue
				}
				repl := fmt.Sprintf("replace %s => %s", r.module, r.relDir)
				out = append(out, repl)
				log.Printf("adding %s => %s", r.module, r.relDir)
			}
		}
	}

	return strings.Join(out, "\n"), nil
}

func realMain() error {
	goModFile := path.Join(*dir, "go.mod")
	if !io.FileExists(goModFile) {
		return errors.Errorf("no go.mod file: %s doesn't exist", goModFile)
	}
	requires, err := findRequires(goModFile)
	if err != nil {
		return err
	}

	goModules, err := findModules(*dir)
	if err != nil {
		return err
	}

	var repls []replacement
	for _, req := range requires {
		for _, goMod := range goModules {
			if goMod.name == req {
				repl := replacement{
					module: goMod.name,
					relDir: goMod.relDir,
				}
				repls = append(repls, repl)
			}
		}
	}

	if len(repls) == 0 {
		log.Printf("no replacements found")
		return nil
	}

	newContent, err := findNewContent(goModFile, repls)
	if err != nil {
		return err
	}
	if *dryRun {
		fmt.Println(newContent)
	} else {
		if err := ioutil.WriteFile(goModFile, []byte(newContent), 0755); err != nil {
			return err
		}
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Dir = *dir
		if err := cmd.Run(); err != nil {
			return errors.Errorf("error running go mod tidy: %v", err)
		}
		log.Printf("wrote to %s", goModFile)
	}

	return nil
}

func main() {
	flag.Parse()
	check.Err(realMain())
}
