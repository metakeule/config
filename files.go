package config

import (
	"os"
	"path/filepath"
)

var (
	USER_DIR    string
	GLOBAL_DIRS string // colon separated list to look for
	WORKING_DIR string
	CONFIG_EXT  = ".conf"
	ENV         []string
	ARGS        []string
)

func init() {
	ENV = os.Environ()
	ARGS = os.Args[1:]
}

// globalsFile returns the global config file path for the given dir
func globalsFile(c *Config, dir string) string {
	return filepath.Join(dir, c.appName(), c.appName()+CONFIG_EXT)
}

// UserFile returns the user defined config file path
func UserFile(c *Config) string {
	return filepath.Join(USER_DIR, c.appName(), c.appName()+CONFIG_EXT)
}

// LocalFile returns the local config file (inside the .config subdir of the current working dir)
func LocalFile(c *Config) string {
	//fmt.Println(WORKING_DIR, ".config", c.appName(), c.appName()+CONFIG_EXT)
	return filepath.Join(WORKING_DIR, ".config", c.appName(), c.appName()+CONFIG_EXT)
}

// GlobalFile returns the path for the global config file in the first global directory
func FirstGlobalsFile(c *Config) string {
	return globalsFile(c, splitGlobals()[0])
}
