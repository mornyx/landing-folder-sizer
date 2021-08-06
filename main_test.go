package main

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalk(t *testing.T) {
	var dir1, dir2 WalkDir
	var err error
	dir1, err = walkSimple(".")
	assert.NoError(t, err)
	wg := sync.WaitGroup{}
	wg.Add(1)
	dirCh := make(chan WalkDir)
	errCh := make(chan error)
	go walk(*flagDir, &wg, dirCh, errCh)
	select {
	case dir2 = <-dirCh:
	case err := <-errCh:
		panic(err)
	}
	assert.Equal(t, dir1.Size, dir2.Size)
}

func TestWalkDir_PrettyPrint(t *testing.T) {
	fmt.Println(WalkDir{
		Name: "aaa",
		Files: []WalkFile{
			{Name: "bbb"},
			{Name: "ccc"},
		},
		Dirs: []WalkDir{{
			Name: "ddd",
			Files: []WalkFile{
				{Name: "fff"},
				{Name: "ggg"},
			}}, {
			Name: "eee",
			Files: []WalkFile{
				{Name: "hhh"},
				{Name: "iii"},
			},
			Dirs: []WalkDir{
				{Name: "jjj"},
			},
		}},
	}.Pretty())
}
