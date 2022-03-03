package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

var validSuffixes = []string{".tsx", ".ts", ".js", ".jsx"}
var apply bool

func main() {
	ignore := ""
	flag.StringVar(&ignore, "ignore", "node_modules", "regex of files to ignore (rule applied before --include)")

	include := ""
	flag.StringVar(&include, "include", `\.(tsx|ts|js|jsx)$`, "regex of files to include")

	dir, _ := os.Getwd()
	if dir != "" {
		dir = filepath.Join(dir, "src")
	}

	flag.StringVar(&dir, "dir", dir, "directory to scan (e.g. <project-root>/src)")
	flag.BoolVar(&apply, "write", false, "writes changes to files (false does a dry-run)")
	flag.Parse()

	fmt.Println("ntbosscher/fix-imports")
	fmt.Println("Author: nate.bosscher@gmail.com")
	fmt.Println("Copyright: 2022")
	fmt.Println()

	if apply {
		fmt.Println("writing changes to disk (write=true)")
		fmt.Println("specify --help for usage and additional options")
	} else {
		fmt.Println("dry-run mode (write=false)")
		fmt.Println("specify --help for usage and additional options")
	}

	fmt.Println()

	ignoreRegex := regexp.MustCompile(ignore)
	matchRegex := regexp.MustCompile(include)

	files := []string{}
	normalizedPaths := []string{}

	log.Println("scanning", dir)
	err := filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if ignoreRegex.MatchString(path) {
			return nil
		}

		if info == nil {
			return nil
		}

		if matchRegex.MatchString(info.Name()) {
			files = append(files, path)
			normalizedPaths = append(normalizedPaths, strings.Join(filepath.SplitList(path), "/"))
		}

		return nil
	})

	if err != nil {
		log.Fatalln(err)
	}

	ctx := &FileContext{
		AllFiles: normalizedPaths,
	}

	log.Println(len(files), "files to process")

	for i, file := range files {
		processFile(ctx, file)

		if i%100 == 0 && i != 0 {
			log.Println(i, "files processed")
		}
	}

	log.Println("done")
}

var importRegex = regexp.MustCompile(`from (["'].*?["']);$`)

func processFile(ctx *FileContext, file string) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Println(file, err)
		return
	}

	lines := bytes.Split(data, []byte("\n"))
	changed := false

	for i, line := range lines {
		if importRegex.Match(line) {

			result := checkImport(ctx, file, line)
			if result != nil {
				lines[i] = result
				changed = true
			}
		}
	}

	if changed && apply {
		err = ioutil.WriteFile(strings.Join(filepath.SplitList(file), "/"), bytes.Join(lines, []byte("\n")), os.ModePerm)
		if err != nil {
			log.Println(file, err)
			return
		}
	}
}

func checkImport(ctx *FileContext, file string, line []byte) []byte {
	matches := importRegex.FindStringSubmatch(string(line))
	name := strings.Trim(matches[1], `"'`)

	if !strings.HasPrefix(name, ".") {
		return nil
	}

	expected := resolve(name, file)
	ok := ctx.fileExistsForImport(expected)
	if ok {
		return nil
	}

	base := strings.TrimLeft(name, "./")
	base = strings.ReplaceAll(base, "../", "") // sometimes there's ../ in the middle of a path
	perms := ctx.partialMatchesFor("/" + base)

	for len(perms) == 0 && strings.Contains(base, "/") {
		parts := strings.Split(base, "/")
		base = strings.Join(parts[1:], "/")
		perms = ctx.partialMatchesFor("/" + base)
	}

	if len(perms) == 1 {
		updated := updateImport(file, perms[0], line, matches[1])
		fmt.Println(matches[1], "->", string(updated))
		return updated
	}

	permsByDistance := map[int][]string{}
	minK := 1000000

	for _, perm := range perms {
		p0 := rel(file, perm)
		k := strings.Count(p0, "/")

		permsByDistance[k] = append(permsByDistance[k], perm)
		if k < minK {
			minK = k
		}
	}

	closePerms := permsByDistance[minK]
	if len(closePerms) == 1 {
		updated := updateImport(file, closePerms[0], line, matches[1])
		fmt.Println(matches[1], "->", string(updated))
		return updated
	}

	if len(perms) == 0 {
		fmt.Println("unable to process import: no options:", matches[1])
		return nil
	}

	fmt.Println(matches, expected, perms)
	log.Fatalln("exit")

	return nil
}

func rel(file string, newImport string) string {
	dir := path.Dir(file)
	ct := 0

	for dir != "/" {
		if strings.HasPrefix(newImport, dir) {
			prefix := "./"
			if ct > 0 {
				prefix = strings.Repeat("../", ct)
			}

			updated := path.Join(prefix, strings.TrimPrefix(newImport, dir))
			if !strings.HasPrefix(updated, "./") && !strings.HasPrefix(updated, "../") {
				updated = "./" + updated
			}

			return updated
		}

		ct++
		dir = path.Dir(dir)

		if ct > 100 {
			log.Fatalln("invalid import:", newImport, "\nfor file:", file)
		}
	}

	log.Fatalln("invalid import:", newImport, "\nfor file:", file)
	return ""
}

func updateImport(file string, newImport string, line []byte, original string) []byte {

	newImport = strings.TrimSuffix(newImport, path.Ext(newImport))
	newImport = strings.TrimSuffix(newImport, "/index")

	value := rel(file, newImport)
	value = `"` + value + `"`
	return bytes.Replace(line, []byte(original), []byte(value), 1)
}

func resolve(importName string, file string) string {
	dir := path.Dir(file)
	return path.Clean(path.Join(dir, importName))
}

type FileContext struct {
	AllFiles []string
}

func (f *FileContext) partialMatchesFor(importName string) []string {
	permutations := f.getSuffixPermutations(importName)
	matches := []string{}

	for _, file := range f.AllFiles {
		if !strings.Contains(file, importName) {
			continue
		}

		for _, perm := range permutations {
			if strings.HasSuffix(file, perm) {
				matches = append(matches, file)
			}
		}
	}

	return matches
}

func (f *FileContext) getSuffixPermutations(importName string) []string {
	list := []string{}

	for _, suffix := range validSuffixes {
		list = append(list, importName+suffix)
	}

	for _, suffix := range validSuffixes {
		list = append(list, path.Join(importName, "index"+suffix))
	}

	return list
}

func (f *FileContext) fileExistsForImport(importName string) bool {
	permutations := f.getSuffixPermutations(importName)

	for _, file := range f.AllFiles {
		if !strings.HasPrefix(file, importName) {
			continue
		}

		for _, match := range permutations {
			if match == file {
				return true
			}
		}

		// found similar but not good enough
	}

	return false
}
