package uifs

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

type FallbackFS struct {
	fs.FS

	prefix    string
	fallback  string
	buildTime time.Time
}

type EmbeddedFile struct {
	fs.File
	io.Seeker

	buildTime time.Time
	seekLock  sync.RWMutex
	seekPtr   int64
	content   []byte
}

type EmbeddedFileInfo struct {
	fs.FileInfo

	buildTime time.Time
}

func NewEmbeddedFile(f fs.File, buildTime time.Time) (*EmbeddedFile, error) {
	ef := &EmbeddedFile{File: f, buildTime: buildTime}

	var err error
	ef.content, err = io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("reading file content: %w", err)
	}

	return ef, nil
}

func (i *EmbeddedFileInfo) ModTime() time.Time {
	return i.buildTime
}

func (f *EmbeddedFile) Stat() (fs.FileInfo, error) {
	stat, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return &EmbeddedFileInfo{FileInfo: stat, buildTime: f.buildTime}, nil
}

func (f *EmbeddedFile) Seek(offset int64, whence int) (int64, error) {
	f.seekLock.Lock()
	defer f.seekLock.Unlock()
	return f.seekUnlocked(offset, whence)
}

func (f *EmbeddedFile) seekUnlocked(offset int64, whence int) (int64, error) {
	var v int64

	switch whence {
	case io.SeekStart:
		v = offset
	case io.SeekCurrent:
		v = f.seekPtr + offset
	case io.SeekEnd:
		v = int64(len(f.content)) - offset
	}

	if v < 0 {
		v = 0
	} else if v > int64(len(f.content)) {
		v = int64(len(f.content))
	}

	f.seekPtr = v
	return v, nil
}

func (f *EmbeddedFile) Read(p []byte) (int, error) {
	f.seekLock.Lock()
	defer f.seekLock.Unlock()

	start := f.seekPtr
	if _, err := f.seekUnlocked(int64(len(p)), io.SeekCurrent); err != nil {
		return 0, err
	}
	end := f.seekPtr

	return copy(p, f.content[start:end]), nil
}

func NewFallbackFS(f fs.FS, prefix, fallback string, buildTime time.Time) *FallbackFS {
	return &FallbackFS{
		FS:        f,
		prefix:    prefix,
		fallback:  filepath.Join(prefix, fallback),
		buildTime: buildTime,
	}
}

func (f *FallbackFS) Open(name string) (fs.File, error) {
	return f.open(name)
}

func (f *FallbackFS) open(name string) (fs.File, error) {
	file, err := f.FS.Open(filepath.Join(f.prefix, name))
	if errors.Is(err, fs.ErrNotExist) {
		file, err = f.FS.Open(f.fallback)
	}
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stating file: %w", err)
	}
	if stat.IsDir() {
		return file, nil
	}

	ef, err := NewEmbeddedFile(file, f.buildTime)
	if err != nil {
		return nil, fmt.Errorf("wrapping embedded file: %w", err)
	}
	return ef, nil
}

type config struct {
	buildTime    time.Time
	prefix       string
	fallbackPath string
}

type Option func(*config)

func WithBuildTime(t time.Time) Option {
	return func(c *config) {
		c.buildTime = t
	}
}

func WithPrefix(prefix string) Option {
	return func(c *config) {
		c.prefix = prefix
	}
}

func WithFallbackPath(path string) Option {
	return func(c *config) {
		c.fallbackPath = path
	}
}

func Handler(efs embed.FS, options ...Option) http.Handler {
	cfg := &config{
		buildTime:    time.Now(),
		fallbackPath: "index.html",
	}

	for _, o := range options {
		o(cfg)
	}

	return http.FileServerFS(NewFallbackFS(efs, cfg.prefix, cfg.fallbackPath, cfg.buildTime))
}
