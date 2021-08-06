package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const indentWidth = 4

var flagDir = flag.String("dir", ".", "dir path to be counted")

// WalkFile represents the file node in the tree.
type WalkFile struct {
	Name string
	Size int64
}

// WalkDir represents the dir node in the tree.
type WalkDir struct {
	Name  string
	Size  int64
	Dirs  []WalkDir
	Files []WalkFile
}

// Pretty returns the pretty output of the tree.
func (d WalkDir) Pretty() string {
	buf := strings.Builder{}
	d.pretty(0, &buf)
	return buf.String()
}

func (d WalkDir) pretty(indent int, buf *strings.Builder) {
	for n := 0; n < indent; n++ {
		d.write(buf, " ")
	}
	d.write(buf, "|- %s (%d)\n", d.Name, d.Size)
	for _, f := range d.Files {
		for n := 0; n < indent+indentWidth; n++ {
			d.write(buf, " ")
		}
		d.write(buf, "|- %s (%d)\n", f.Name, f.Size)
	}
	for _, dir := range d.Dirs {
		dir.pretty(indent+indentWidth, buf)
	}
}

func (d WalkDir) write(buf *strings.Builder, format string, a ...interface{}) {
	_, _ = fmt.Fprintf(buf, format, a...)
}

// walkSimple traverses the folders and builds the tree in a single goroutine.
// Just for comparison.
func walkSimple(path string) (WalkDir, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return WalkDir{}, err
	}
	infos, err := ioutil.ReadDir(path)
	if err != nil {
		return WalkDir{}, err
	}
	var dirs []WalkDir
	var files []WalkFile
	size := info.Size()
	for _, info := range infos {
		if info.IsDir() {
			dir, err := walkSimple(filepath.Join(path, info.Name()))
			if err != nil {
				return WalkDir{}, err
			}
			size += dir.Size
			dirs = append(dirs, dir)
		} else {
			size += info.Size()
			files = append(files, WalkFile{
				Name: info.Name(),
				Size: info.Size(),
			})
		}
	}
	return WalkDir{
		Name:  info.Name(),
		Size:  size,
		Dirs:  dirs,
		Files: files,
	}, nil
}

// walk traverses the folders and builds the tree in multi-goroutines.
// The caller is responsible for building wg and needs to call wg.Wait()
// externally to wait for the call to finish.
// WalkDir will be consumed from dirCh if successful, otherwise error
// will be consumed from errCh.
func walk(path string, wg *sync.WaitGroup, dirCh chan<- WalkDir, errCh chan<- error) {
	defer wg.Done()

	// Get dir info.
	info, err := os.Lstat(path)
	if err != nil {
		errCh <- err
		return
	}

	// Read sub files & dirs.
	infos, err := ioutil.ReadDir(path) // TODO: limit read concurrency.
	if err != nil {
		errCh <- err
		return
	}

	// Iterate sub entries, if it is a file type, append to slice and add size directly,
	// otherwise start a new goroutine for asynchronous statistics.
	var dirs []WalkDir
	var files []WalkFile
	size := info.Size()
	subWg := sync.WaitGroup{}
	subDirCh := make(chan WalkDir)
	subErrCh := make(chan error)
	for _, info := range infos {
		if info.IsDir() {
			subWg.Add(1)
			go walk(filepath.Join(path, info.Name()), &subWg, subDirCh, subErrCh)
		} else {
			size += info.Size()
			files = append(files, WalkFile{
				Name: info.Name(),
				Size: info.Size(),
			})
		}
	}

	go func() {
		// Wait until all sub dirs are walked, then close the
		// channel to notify main goroutine that it is over.
		subWg.Wait()
		close(subDirCh)
		close(subErrCh)
	}()

Loop:
	for {
		select {
		case dir, ok := <-subDirCh:
			if !ok {
				// Here we know that all sub dirs have been walked.
				break Loop
			}
			size += dir.Size
			dirs = append(dirs, dir)
		case err, ok := <-subErrCh:
			if !ok {
				break Loop
			}
			errCh <- err
		}
	}

	// Notifies the caller of the aggregated result.
	dirCh <- WalkDir{
		Name:  info.Name(),
		Size:  size,
		Dirs:  dirs,
		Files: files,
	}
}

func main() {
	flag.Parse()
	wg := sync.WaitGroup{}
	wg.Add(1)
	dirCh := make(chan WalkDir)
	errCh := make(chan error)
	go walk(*flagDir, &wg, dirCh, errCh)
	select {
	case dir := <-dirCh:
		fmt.Println(dir.Pretty())
	case err := <-errCh:
		panic(err)
	}
}
