package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/mholt/archiver/v4"
)

var (
	dest             string
	skipEmptySymlink bool
)

func init() {
	flag.StringVar(&dest, "dest", ".", "destination")
	flag.BoolVar(&skipEmptySymlink, "skip-empty-symlink", true, "skip empty symlink")
}

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

type seekerReadAt interface {
	io.ReaderAt
	io.Seeker
}

func run() error {
	format, r, err := archiver.Identify("", os.Stdin)
	if err != nil {
		return err
	}

	if _, ok := r.(seekerReadAt); !ok {
		b, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}

	ex, ok := format.(archiver.Extractor)
	if !ok {
		return fmt.Errorf("%s is not extractable", format.Name())
	}
	if err := ex.Extract(context.Background(), r, nil, func(ctx context.Context, f archiver.File) error {
		fi := f.FileInfo
		p := filepath.Join(dest, f.NameInArchive)
		mode := fi.Mode() &^ 0o022
		if f.IsDir() {
			if err := os.MkdirAll(p, mode); err != nil {
				return err
			}
			return nil
		}
		if mode&fs.ModeSymlink == fs.ModeSymlink {
			if f.LinkTarget == "" {
				if skipEmptySymlink {
					return nil
				}
				return fmt.Errorf("symlink is not supported for %s", format.Name())
			}
			if err := os.Symlink(f.LinkTarget, p); err != nil {
				return err
			}
			return nil
		}

		dst, err := os.Create(p)
		if err != nil {
			return err
		}
		defer dst.Close()
		if err := dst.Chmod(mode); err != nil {
			return err
		}
		r, err := f.Open()
		if err != nil {
			return err
		}
		if _, err := io.Copy(dst, r); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}
