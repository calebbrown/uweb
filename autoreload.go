// Copyright 2013 Caleb Brown. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package uweb

import (
	"container/list"
	"fmt"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime"
	"time"
)

const (
	BUILD_FILE string = "autoreload.out"
	LOCK_FILE  string = "autoreload.lock"
)

// gatherFiles finds and traverses all the imports to identify the files
// we need to watch for changes.
func gatherFiles(file string) []string {
	packages := make(map[string]*build.Package)
	imports := list.New()

	files := map[string]bool{file: true}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
	if err != nil {
		panic(err)
	}
	for _, s := range f.Imports {
		v := s.Path.Value
		imports.PushBack(v[1 : len(v)-1])
	}

	for e := imports.Front(); e != nil; e = e.Next() {
		i := e.Value.(string)
		if _, ok := packages[i]; ok {
			continue
		}
		p, _ := build.Import(i, ".", build.AllowBinary)
		packages[i] = p
		for _, l := range [][]string{p.GoFiles, p.CgoFiles, p.CFiles, p.HFiles,
			p.SFiles, p.SysoFiles} {
			for _, f := range l {
				files[path.Join(p.Dir, f)] = true
			}
		}
		for _, i := range p.Imports {
			imports.PushBack(i)
		}
	}

	returnFiles := make([]string, len(files))
	i := 0
	for f := range files {
		returnFiles[i] = f
		i++
	}

	return returnFiles
}

// findMainFile walks the call graph to find "main.main" which is where
// we start our dependency traversing from
func findMainFile() string {
	skip := 0
	for {
		pc, file, _, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		fun := runtime.FuncForPC(pc)
		if fun.Name() == "main.main" {
			return file
		}
		skip++
	}
	panic("No main() found")
}

// fileChanged will monitor a list of files and return when the mod time
// is newer than a given time.
func fileChanged(files []string, modTime time.Time) bool {
	for _, file := range files {
		fs, _ := os.Lstat(file)
		if fs.ModTime().After(modTime) {
			return true
		}
	}
	return false
}

func runAndWatch(file string) (bool, error) {
	// Build the executable
	buildCmd := exec.Command("go", "build", "-o", BUILD_FILE, file)
	buildCmd.Stderr = os.Stderr
	buildCmd.Stdout = os.Stdout
	buildSuccess := true
	if err := buildCmd.Run(); err != nil {
		fmt.Println("\nFailed to rebuild. Waiting...\n")
		buildSuccess = false
	}

	// Run the executable if it built successfully
	cmd := exec.Command("./"+BUILD_FILE, os.Args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if buildSuccess {
		if err := cmd.Start(); err != nil {
			return false, err
		}

		// Setup a thread to release the cmd when it's finished
		go func() {
			cmd.Wait()
		}()
	}

	// Prep the monitoring
	files := gatherFiles(file)
	os.Truncate(LOCK_FILE, 0)
	fs, _ := os.Lstat(LOCK_FILE)

	// Monitor loop
	// Aborts when the process quits or a dependency changes
	for !fileChanged(files, fs.ModTime()) {
		if buildSuccess && cmd.ProcessState != nil {
			return false, nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Make sure the process is really finished
	if buildSuccess {
		err := cmd.Process.Kill()
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func AutoReloader() {
	// Attempt to create a file
	// If we can then run the auto reload, if not fall through to the normal code
	lockFile, err := os.OpenFile(LOCK_FILE, os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return
	}
	lockFile.Close()

	cleanUp := func() {
		os.Remove(LOCK_FILE)
		os.Remove(BUILD_FILE)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		cleanUp()
	}()
	defer func() {
		cleanUp()
		os.Exit(0)
	}()

	mainFile := findMainFile()
	fmt.Println("Starting...")
	for {
		ok, err := runAndWatch(mainFile)
		if err != nil {
			fmt.Println(err)
			return
		}
		if !ok {
			break
		}

		fmt.Println("\nChange detected. Restarting...\n")
	}
}
