package store

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gobuffalo/packr/v2/jam/parser"
)

func Clean(root string) error {
	defer func() {
		packd := filepath.Join(root, "packrd")
		os.RemoveAll(packd)
	}()

	p, err := parser.NewFromRoots([]string{root}, &parser.RootsOptions{})
	if err != nil {
		return err
	}

	boxes, err := p.Run()
	if err != nil {
		return err
	}

	d := NewDisk("", "")
	for _, box := range boxes {
		if err := d.Clean(box); err != nil {
			return err
		}
	}
	return nil
}

func clean(root string) error {
	if len(root) == 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return err
		}
		root = pwd
	}
	if _, err := os.Stat(root); err != nil {
		return nil
	}
	defer func() {
		packd := filepath.Join(root, "packrd")
		os.RemoveAll(packd)
	}()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if filepath.Base(path) == "packrd" {
				os.RemoveAll(path)
				return filepath.SkipDir
			}
		}
		if strings.HasSuffix(path, "-packr.go") {
			err := os.RemoveAll(path)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}
