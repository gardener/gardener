package jam

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/gobuffalo/packr/v2/jam/parser"
	"github.com/gobuffalo/packr/v2/jam/store"
	"github.com/gobuffalo/packr/v2/plog"
)

// PackOptions ...
type PackOptions struct {
	IgnoreImports bool
	Legacy        bool
	StoreCmd      string
	Roots         []string
}

// Pack the roots given + PWD
func Pack(opts PackOptions) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	opts.Roots = append(opts.Roots, pwd)
	if err := Clean(opts.Roots...); err != nil {
		return err
	}

	p, err := parser.NewFromRoots(opts.Roots, &parser.RootsOptions{
		IgnoreImports: opts.IgnoreImports,
	})
	if err != nil {
		return err
	}
	boxes, err := p.Run()
	if err != nil {
		return err
	}

	// reduce boxes - remove ones we don't want
	// MB: current assumption is we want all these
	// boxes, just adding a comment suggesting they're
	// might be a reason to exclude some

	plog.Logger.Debugf("found %d boxes", len(boxes))

	if len(opts.StoreCmd) != 0 {
		return ShellPack(opts, boxes)
	}

	var st store.Store = store.NewDisk("", "")

	if opts.Legacy {
		st = store.NewLegacy()
	}

	for _, b := range boxes {
		if b.Name == store.DISK_GLOBAL_KEY {
			continue
		}
		if err := st.Pack(b); err != nil {
			return err
		}
	}
	if cl, ok := st.(io.Closer); ok {
		return cl.Close()
	}
	return nil
}

// ShellPack ...
func ShellPack(opts PackOptions, boxes parser.Boxes) error {
	b, err := json.Marshal(boxes)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, opts.StoreCmd, string(b))
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()

}

// Clean ...
func Clean(args ...string) error {
	pwd, err := os.Getwd()
	if err != nil {
		return err
	}
	args = append(args, pwd)
	for _, root := range args {
		if err := store.Clean(root); err != nil {
			return err
		}
	}
	return nil
}
