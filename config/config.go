// Package config has gocode config file reading and writing.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"

	"github.com/BurntSushi/toml"
)

// Config houses configuration settings and corresponds to a TOML file.
type Config struct {
	Settings map[string]interface{}
}

// GetString returns an entry as a string or def if not found.
func (c *Config) GetString(key string, def string) string {
	v, ok := c.Settings[key]
	if !ok {
		return def
	}
	return fmt.Sprint(v)
}

// ReadFrom implements the ReaderFrom interface and reads and parses a TOML file.
func (c *Config) ReadFrom(r io.Reader) (n int64, err error) {

	b, err := ioutil.ReadAll(r)
	n = int64(len(b))
	if err != nil {
		return n, err
	}

	if c.Settings == nil {
		c.Settings = make(map[string]interface{})
	}
	err = toml.Unmarshal(b, &c.Settings)
	return n, err

}

// WriteTo implements the WriterTo interafce and writes out a TOML file.
func (c *Config) WriteTo(w io.Writer) (n int64, err error) {

	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	err = enc.Encode(&c.Settings)
	if err != nil {
		return 0, err
	}

	wn, err := w.Write(buf.Bytes())
	return int64(wn), err
}

// LoadFS will look for the config file in .gocode/gocode.toml and load it.
// If emptyIfNotFound is true then the case of the file not being present
// returns empty config instead of an error, or if it is false then
// the underlying filesystem's not found error will be returned.
func LoadFS(moduleFS fs.FS, emptyIfNotFound bool) (*Config, error) {
	f, err := moduleFS.Open(".gocode/gocode.toml")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && emptyIfNotFound {
			return &Config{Settings: make(map[string]interface{})}, nil
		}
		return nil, err
	}
	defer f.Close()
	var c Config
	_, err = c.ReadFrom(f)
	return &c, err
}

// StoreFS writes a Config to the .gocode/gocode.toml.
// The filesystem must provide an appropriate WriteFile
// and MkdirAll methods.
// (see FileWriter and MkdirAller interfaces).
func StoreFS(moduleFS fs.FS, c *Config) error {
	fw, ok := moduleFS.(wfs)
	if !ok {
		return errors.New("moduleFS must implement FileWriter and MkdirAller")
	}

	err := fw.MkdirAll(".gocode", 0755)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	_, err = c.WriteTo(&buf)
	if err != nil {
		return err
	}

	return fw.WriteFile(".gocode/gocode.toml", buf.Bytes(), 0644)
}

// FileWriter is an FS that has a WriteFile method on it.
type FileWriter interface {
	// memfs and other implementations should provide such a method to support writes
	WriteFile(name string, data []byte, perm fs.FileMode) error
}

// MkdirAller has a MkdirAll call matching the os.MkdirAll signature.
type MkdirAller interface {
	MkdirAll(path string, perm os.FileMode) error
}

type wfs interface {
	FileWriter
	MkdirAller
}
