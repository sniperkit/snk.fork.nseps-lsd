/*
Sniperkit-Bot
- Status: analyzed
*/

// Author: Nikolas Sepos <nikolas.sepos@gmail.com>
//
// SPDX-License-Identifier: MIT
//

package main

import (
	"bufio"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nseps/godag"
	"github.com/spf13/pflag"
)

// Data is exported so i can use data as variable name
type Data struct {
	Name string
	Path string
}

var ldPath []string

func main() {

	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "* Usage of %s:\n", os.Args[0])
		pflag.PrintDefaults()
	}

	libPath := pflag.StringP("libPath", "L", "", "A colon separated list of paths to look for libraries. If set this would be the only path")
	appLibPath := pflag.StringP("appendlibPath", "a", "", "A colon separated list of paths to append in default path")
	preLibPath := pflag.StringP("prependlibPath", "p", "", "A colon separated list of paths to prepend in default path")
	ldConf := pflag.String("ldConf", "/etc/ld.so.conf", "Parse ld config file and append to the default path")

	trace := pflag.Bool("trace", false, "Run binary with LD_TRACE_LOADED_OBJECTS=1 set. This is equivalent to ldd <target>. No need for ldd binary")
	tree := pflag.Bool("tree", false, "Get a nice dependency tree")
	export := pflag.String("export", "", "Export directory path. It will copy everything it finds, including target binary, to dir")

	pflag.Parse()
	// we want 1 argument. the target binary
	if len(os.Args) < 2 {
		die(fmt.Errorf("path to binary is missing"))
	}
	target := os.Args[1]
	// chack if exists
	if _, err := os.Stat(target); err != nil {
		die(fmt.Errorf("cannot stat target binary: %v", err))
	}
	// parse elf
	file, err := elf.Open(target)
	if err != nil {
		die(fmt.Errorf("cannot read elf target: %v", err))
	}
	defer file.Close()

	// simulate ldd <target>
	if *trace {
		cmd := exec.Command(target)
		cmd.Env = []string{"LD_TRACE_LOADED_OBJECTS=1"}
		out, err := cmd.CombinedOutput()
		if err != nil {
			die(fmt.Errorf("could not execute target %s: %v", target, err))
		}
		// what's our target
		fmt.Printf("Target: %s, Class: %s\n", target, file.Class)
		// print ldd output
		fmt.Print(string(out))
		return
	}

	if *libPath == "" {
		// prepend to path
		if *preLibPath != "" {
			ldPath = strings.Split(*preLibPath, ":")
		}
		// binary path
		binPath := filepath.Dir(target)
		ldPath = append(ldPath, binPath)
		// prepend /lib64 if is 64bit bin
		if file.Class == elf.ELFCLASS64 {
			ldPath = append(ldPath, "/lib64")
		}
		// default lib path
		ldPath = append(ldPath, "/lib", "/usr/lib")
		// append to path
		if *appLibPath != "" {
			ldPath = append(ldPath, strings.Split(*appLibPath, ":")...)
		}
		// parse ld conf
		if *ldConf != "" {
			pathLdConf, err := parseLdConf(*ldConf)
			if err != nil {
				// no need to die on this
				fmt.Printf("Warning: Parsing ldconfig failed: %v\n", err)
			} else {
				ldPath = append(ldPath, pathLdConf...)
			}
		}
	} else {
		// there shall be only one path!
		ldPath = strings.Split(*libPath, ":")
	}

	fmt.Println("Path lookup order:")
	fmt.Println(strings.Join(ldPath, "\n"))
	fmt.Println()

	d := dag.New()

	err = getTree(target, filepath.Base(target), d)
	if err != nil {
		die(fmt.Errorf("failed to get tree: %v", err))
	}

	root := d.Roots()[0]
	deps := map[string]*Data{}

	root.Walk(dag.WalkDepthFirst, func(node *dag.Node, depth int) error {
		// get data from graph
		data, ok := node.Value.(*Data)
		if !ok {
			die(fmt.Errorf("Cannot get value from node"))
		}
		// TODO: Expose the underlaying map from godag
		//       to not have to walk and get unique keys.
		//       We know that the map has already all we need.
		if _, inMap := deps[data.Name]; !inMap {
			deps[data.Name] = data
		}
		// if tree should be surrounding the Walk function
		// after the above is solved.
		if *tree {
			// print tree
			for i := 0; i < depth; i = i + 1 {
				fmt.Printf("  |")
			}
			fmt.Printf("-%s\n", data.Name)
		}
		return nil
	})

	if *export != "" {
		os.Mkdir(*export, 0775)
		for _, dep := range deps {
			if dep.Path == "" {
				fmt.Printf("Warning: File not found for lib %s\n", dep.Name)
				continue
			}
			fmt.Printf("Copy %s: %s => %s\n", dep.Name, dep.Path, *export)
			if err := copyFile(dep.Path, filepath.Join(*export, dep.Name)); err != nil {
				die(fmt.Errorf("cannot copy file %s: %v", dep.Path, err))
			}
		}
		return
	}

	if !*tree {
		// what's our target
		fmt.Printf("Target: %s, Class: %s\n", target, file.Class)
		for _, dep := range deps {
			// just print the unsorted map
			fmt.Printf("  %s => %s\n", dep.Name, dep.Path)
		}
	}
}

func die(err error) {
	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	pflag.Usage()
	os.Exit(1)

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

func getTree(path, name string, d *dag.Dag) error {

	deps, err := getLibDeps(path)
	if err != nil {
		return err

	}
	d.AddNode(name, &Data{
		Name: name,
		Path: path,
	})

	for _, lib := range deps {
		libFile, err := findInPath(lib, ldPath)
		if err != nil {
			return err
		}
		// we could not find the lib
		if libFile == "" {
			// just add the node
			d.AddNode(lib, &Data{
				Name: lib,
				Path: "",
			})
		} else {
			// recurse
			err := getTree(libFile, lib, d)
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
