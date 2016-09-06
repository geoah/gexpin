package main

import (
	"path/filepath"

	gx "github.com/whyrusleeping/gx/gxutil"
)

const PkgFileName = gx.PkgFileName

func LoadPackageFile(path string) (*gx.Package, error) {
	if path == PkgFileName {
		root, err := gx.GetPackageRoot()
		if err != nil {
			return nil, err
		}

		path = filepath.Join(root, PkgFileName)
	}

	var pkg gx.Package
	err := gx.LoadPackageFile(&pkg, path)
	if err != nil {
		return nil, err
	}

	if pkg.GxVersion == "" {
		pkg.GxVersion = gx.GxVersion
	}

	return &pkg, nil
}

func NewGxPM() (*gx.PM, error) {
	cfg, err := gx.LoadConfig()
	if err != nil {
		return nil, err
	}

	return gx.NewPM(cfg)
}
