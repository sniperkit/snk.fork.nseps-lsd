package main

import (
	"bufio"
	"debug/elf"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nseps/godag"
)

type Data struct {
	Name string
	Path string
}

func main() {

	if len(os.Args) < 2 {
		log.Fatalf("Error: Path to binary is missing\n")
	}
	target := os.Args[1]

	if _, err := os.Stat(target); err != nil {
		log.Fatalf("Cannot stat target binary: %v\n", err)
	}

	file, err := elf.Open(target)
	if err != nil {
		log.Fatalf("Cannot read elf target: %v\n", err)
	}
	// default lib path
	path := []string{"/lib", "/usr/lib"}
	// prepend /lib64 if is 64bit bin
	if file.Class == elf.ELFCLASS64 {
		path = append([]string{"/lib64"}, path...)
	}

	pathLdConf, err := parseLdConf("/etc/ld.so.conf")
	if err != nil {
		log.Fatalf("Parsing ldconfig failed: %v\n", err)
	}
	path = append(path, pathLdConf...)

	binPath := filepath.Dir(target)
	path = append(path, binPath)

	d := dag.New()

	err = getTree(target, filepath.Base(target), &path, d)

	root := d.Roots()[0]
	deps := map[string]*Data{}

	root.Walk(dag.WalkDepthFirst, func(node *dag.Node, depth int) error {
		data, ok := node.Value.(*Data)
		if !ok {
			log.Fatalf("Cannot get value from node")
		}

		if _, inMap := deps[data.Name]; !inMap {
			deps[data.Name] = data
		}
		return nil
	})

	os.Mkdir("out", 0775)
	for _, dep := range deps {
		fmt.Printf("%s => %s\n", dep.Name, dep.Path)
		if err := copyFile(dep.Path, "out/"+dep.Name); err != nil {
			log.Fatalf("Cannot copy file %s: %v\n", dep.Path, err)
		}
	}

}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	inStat, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, inStat.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func getTree(path, name string, ldPath *[]string, d *dag.Dag) error {

	deps, err := getLibDeps(path)
	if err != nil {
		return err

	}
	d.AddNode(name, &Data{
		Name: name,
		Path: path,
	})

	for _, lib := range deps {
		libFile, err := findInPath(lib, *ldPath)
		if err != nil {
			return err
		}

		if libFile == "" {
			// just add the node
			d.AddNode(lib, &Data{
				Name: lib,
				Path: "",
			})
		} else {
			// recurse
			err := getTree(libFile, lib, ldPath, d)
			if err != nil {
				return err
			}
		}
		d.AddEdge(name, lib)
		if err != nil {
			fmt.Println(err)
		}
	}
	return nil
}

func getLibDeps(path string) ([]string, error) {
	file, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	deps, err := file.ImportedLibraries()
	if err != nil {
		return nil, err

	}
	return deps, nil
}

func findInPath(lib string, path []string) (string, error) {
	for _, p := range path {

		file := filepath.Join(p, lib)
		_, err := os.Stat(file)

		if err == nil {
			return file, nil
		}
		if os.IsNotExist(err) {
			continue
		}
		return "", err
	}
	return "", nil
}

var rInclude = regexp.MustCompile("^include (.*)$")

func parseLdConf(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ret := []string{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// discard empty lines
		if strings.TrimLeft(line, " ") == "" {
			continue
		}
		// discard comments
		if strings.TrimLeft(line, " ")[0] == '#' {
			continue
		}
		// include other files
		if rInclude.MatchString(line) {
			// grub the file glob
			match := rInclude.FindStringSubmatch(line)
			// get included file paths
			incPaths, err := filepath.Glob(match[1])
			if err != nil {
				return nil, err
			}
			for _, incPath := range incPaths {
				inc, err := parseLdConf(incPath)
				if err != nil {
					return nil, err
				}
				ret = append(ret, inc...)
			}
			continue
		}
		ret = append(ret, filepath.Clean(line))
	}
	return ret, nil
}

func genString(i int, c byte) string {
	b := make([]byte, i)
	for j := range b {
		b[j] = c
	}
	return string(b)
}
