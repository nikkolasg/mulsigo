package util

import (
	"errors"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

var defaultHome string

type Tomler interface {
	Toml() interface{}
	FromToml(string) error
}

func init() {
	u, err := user.Current()
	if err != nil {
		panic(err)
	}
	defaultHome = u.HomeDir
}

func DefaultConfigPath(app string) string {
	return filepath.Join(defaultHome, "."+app)
}

type Config struct {
	parent *Config
	dir    string
}

func NewConfig(app string) *Config {
	return NewConfigWithPath(DefaultConfigPath(app))
}

func NewConfigWithPath(path string) *Config {
	return &Config{
		dir: path,
	}
}

func (c *Config) Path() string {
	return c.dir
}

func (c *Config) Dir(folder string) *Config {
	return &Config{c, filepath.Join(c.dir, folder)}
}

func (c *Config) RelPath(fname string) string {
	root := c.root()
	rel, err := filepath.Rel(root.dir, filepath.Join(c.dir, fname))
	if err != nil {
		panic(err)
	}
	return rel
}

func (c *Config) root() *Config {
	if c.parent == nil {
		return c
	}
	return c.parent.root()
}

func (c *Config) Write(fname string, i interface{}) error {
	c.createFolderIfNotExists()
	path := filepath.Join(c.dir, fname)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	toTomlize := i
	if t, ok := i.(Tomler); ok {
		toTomlize = t.Toml()
	}
	enc := toml.NewEncoder(file)
	return enc.Encode(toTomlize)
}

func (c *Config) Read(fname string, i interface{}) error {
	c.createFolderIfNotExists()
	path := filepath.Join(c.dir, fname)
	ex, err := exists(path)
	if !ex || err != nil {
		return errors.New("config: file " + fname + " does not exists")
	}

	if tomler, ok := i.(Tomler); ok {
		buff, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		return tomler.FromToml(string(buff))
	}

	_, err = toml.DecodeFile(path, i)
	return err
}

func (c *Config) createFolderIfNotExists() {
	ex, err := exists(c.dir)
	if ex || err != nil {
		return
	}
	if err := os.MkdirAll(c.dir, 0700); err != nil {
		panic(err)
	}
}

func (c *Config) List(match string) []string {
	m, err := filepath.Glob(filepath.Join(c.dir, match))
	if err != nil {
		panic(err)
	}
	for i := range m {
		_, f := filepath.Split(m[i])
		m[i] = f
	}
	return m
}

func (c *Config) ListDir() ([]string, error) {
	fi, err := ioutil.ReadDir(c.dir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, f := range fi {
		if f.IsDir() {
			dirs = append(dirs, f.Name())
		}
	}
	return dirs, nil
}

// exists returns whether the given file or directory exists or not
func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}
