package store

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"fmt"
	"go/build"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gobuffalo/envy"
	"github.com/gobuffalo/genny"
	"github.com/gobuffalo/gogen"
	"github.com/gobuffalo/packr/v2/file/resolver/encoding/hex"
	"github.com/gobuffalo/packr/v2/plog"
	"github.com/rogpeppe/go-internal/modfile"

	"github.com/pkg/errors"

	"github.com/gobuffalo/packr/v2/jam/parser"
	"github.com/karrick/godirwalk"
	"golang.org/x/sync/errgroup"
)

var _ Store = &Disk{}

const DISK_GLOBAL_KEY = "__packr_global__"

type Disk struct {
	DBPath    string
	DBPackage string
	global    map[string]string
	boxes     map[string]*parser.Box
	moot      *sync.RWMutex
}

func NewDisk(path string, pkg string) *Disk {
	if len(path) == 0 {
		path = "packrd"
	}
	if len(pkg) == 0 {
		pkg = "packrd"
	}
	if !filepath.IsAbs(path) {
		path, _ = filepath.Abs(path)
	}
	return &Disk{
		DBPath:    path,
		DBPackage: pkg,
		global:    map[string]string{},
		boxes:     map[string]*parser.Box{},
		moot:      &sync.RWMutex{},
	}
}

func (d *Disk) FileNames(box *parser.Box) ([]string, error) {
	path := box.AbsPath
	if len(box.AbsPath) == 0 {
		path = box.Path
	}
	var names []string
	if _, err := os.Stat(path); err != nil {
		return names, nil
	}
	err := godirwalk.Walk(path, &godirwalk.Options{
		FollowSymbolicLinks: true,
		Callback: func(path string, de *godirwalk.Dirent) error {
			if !de.IsRegular() {
				return nil
			}
			names = append(names, path)
			return nil
		},
	})
	return names, err
}

func (d *Disk) Files(box *parser.Box) ([]*parser.File, error) {
	var files []*parser.File
	names, err := d.FileNames(box)
	if err != nil {
		return files, err
	}
	for _, n := range names {
		b, err := ioutil.ReadFile(n)
		if err != nil {
			return files, err
		}
		f := parser.NewFile(n, bytes.NewReader(b))
		files = append(files, f)
	}
	return files, nil
}

func (d *Disk) Pack(box *parser.Box) error {
	plog.Debug(d, "Pack", "box", box.Name)
	d.boxes[box.Name] = box
	names, err := d.FileNames(box)
	if err != nil {
		return err
	}
	for _, n := range names {
		_, ok := d.global[n]
		if ok {
			continue
		}
		k := makeKey(box, n)
		// not in the global, so add it!
		d.global[n] = k
	}
	return nil
}

func (d *Disk) Clean(box *parser.Box) error {
	root := box.PackageDir
	if len(root) == 0 {
		return errors.New("can't clean an empty box.PackageDir")
	}
	plog.Debug(d, "Clean", "box", box.Name, "root", root)
	return clean(root)
}

type options struct {
	Package     string
	GlobalFiles map[string]string
	Boxes       []optsBox
	GK          string
}

type optsBox struct {
	Name string
	Path string
}

func (d *Disk) Generator() (*genny.Generator, error) {
	g := genny.New()
	if len(d.boxes) == 0 {
		return g, nil
	}

	xb := &parser.Box{Name: DISK_GLOBAL_KEY}
	opts := options{
		Package:     d.DBPackage,
		GlobalFiles: map[string]string{},
		GK:          makeKey(xb, d.DBPath),
	}

	wg := errgroup.Group{}
	for k, v := range d.global {
		func(k, v string) {
			wg.Go(func() error {
				bb := &bytes.Buffer{}
				enc := hex.NewEncoder(bb)
				zw := gzip.NewWriter(enc)
				f, err := os.Open(k)
				if err != nil {
					return err
				}
				defer f.Close()
				io.Copy(zw, f)
				if err := zw.Close(); err != nil {
					return err
				}
				d.moot.Lock()
				opts.GlobalFiles[makeKey(xb, k)] = bb.String()
				d.moot.Unlock()
				return nil
			})
		}(k, v)
	}

	if err := wg.Wait(); err != nil {
		return g, err
	}

	for _, b := range d.boxes {
		ob := optsBox{
			Name: b.Name,
		}
		opts.Boxes = append(opts.Boxes, ob)
	}

	sort.Slice(opts.Boxes, func(a, b int) bool {
		return opts.Boxes[a].Name < opts.Boxes[b].Name
	})

	t := gogen.TemplateTransformer(opts, template.FuncMap{
		"printBox": func(ob optsBox) (template.HTML, error) {
			box := d.boxes[ob.Name]
			if box == nil {
				return "", errors.Errorf("could not find box %s", ob.Name)
			}
			fn, err := d.FileNames(box)
			if err != nil {
				return "", err
			}
			if len(fn) == 0 {
				return "", nil
			}

			type file struct {
				Resolver    string
				ForwardPath string
			}

			gf := genny.NewFile("box.go.tmpl", strings.NewReader(diskGlobalBoxTmpl))
			var files []file
			for _, s := range fn {
				p := strings.TrimPrefix(s, box.AbsPath)
				p = strings.TrimPrefix(p, string(filepath.Separator))
				files = append(files, file{
					Resolver:    strings.Replace(p, "\\", "/", -1),
					ForwardPath: makeKey(box, s),
				})
			}
			opts := map[string]interface{}{
				"Box":   box,
				"Files": files,
			}
			t := gogen.TemplateTransformer(opts, nil)
			gf, err = t.Transform(gf)
			if err != nil {
				return "", err
			}
			return template.HTML(gf.String()), nil
		},
	})
	g.Transformer(t)

	fp := filepath.Join(d.DBPath, "packed-packr.go.tmpl")
	global := genny.NewFile(fp, strings.NewReader(diskGlobalTmpl))
	global, err := t.Transform(global)
	if err != nil {
		return g, err
	}

	ft := gogen.FmtTransformer()
	global, err = ft.Transform(global)
	if err != nil {
		return g, err
	}
	g.File(global)

	var ip string
	if envy.Mods() {
		// Starting in 1.12, we can rely on Go's method for
		// resolving where go.mod resides. Prior versions will
		// simply return an empty string.
		cmd := exec.Command("go", "env", "GOMOD")
		out, err := cmd.Output()
		if err != nil {
			return g, errors.New("go.mod cannot be read or does not exist while go module is enabled")
		}
		mp := strings.TrimSpace(string(out))
		if mp == "" {
			// We are on a prior version of Go; try and do
			// the resolution ourselves.
			mp = filepath.Join(filepath.Dir(d.DBPath), "go.mod")
			if _, err := os.Stat(mp); err != nil {
				mp = filepath.Join(d.DBPath, "go.mod")
			}
		}

		moddata, err := ioutil.ReadFile(mp)
		if err != nil {
			return g, errors.New("go.mod cannot be read or does not exist while go module is enabled.")
		}
		ip = modfile.ModulePath(moddata)
		if ip == "" {
			return g, errors.New("go.mod is malformed.")
		}
		ip = filepath.Join(ip, strings.TrimPrefix(filepath.Dir(d.DBPath), filepath.Dir(mp)))
		ip = strings.Replace(ip, "\\", "/", -1)
	} else {
		ip = filepath.Dir(d.DBPath)
		srcs := envy.GoPaths()
		srcs = append(srcs, build.Default.SrcDirs()...)
		for _, x := range srcs {
			ip = strings.TrimPrefix(ip, "/private")
			ip = strings.TrimPrefix(ip, x)
		}
		ip = strings.TrimPrefix(ip, string(filepath.Separator))
		ip = strings.TrimPrefix(ip, "src")
		ip = strings.TrimPrefix(ip, string(filepath.Separator))

		ip = strings.Replace(ip, "\\", "/", -1)
	}
	ip = path.Join(ip, d.DBPackage)

	for _, n := range opts.Boxes {
		b := d.boxes[n.Name]
		if b == nil {
			continue
		}
		p := filepath.Join(b.PackageDir, b.Package+"-packr.go.tmpl")
		f := genny.NewFile(p, strings.NewReader(diskImportTmpl))

		o := struct {
			Package string
			Import  string
		}{
			Package: b.Package,
			Import:  ip,
		}

		t := gogen.TemplateTransformer(o, template.FuncMap{})
		f, err := t.Transform(f)
		if err != nil {
			return g, nil
		}
		g.File(f)
	}

	return g, nil
}

func (d *Disk) Close() error {
	plog.Debug(d, "Close")
	run := genny.WetRunner(context.Background())
	run.Logger = plog.Logger
	if err := run.WithNew(d.Generator()); err != nil {
		return err
	}
	return run.Run()
}

// resolve file paths (only) for the boxes
// compile "global" db
// resolve files for boxes to point at global db
// write global db to disk (default internal/packr)
// write boxes db to disk (default internal/packr)
// write -packr.go files in each package (1 per package) that init the global db

func makeKey(box *parser.Box, path string) string {
	w := md5.New()
	fmt.Fprint(w, path)
	h := hex.EncodeToString(w.Sum(nil))
	return h
}
