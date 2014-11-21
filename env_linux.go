// +build linux

// set USER_DIR, GLOBAL_DIRS and WORKING_DIR based on the XDG Base Directory Specification
// see http://standards.freedesktop.org/basedir-spec/basedir-spec-latest.html
package config

import "os"

func setUserDir() {
	xdg_config_home := os.Getenv("XDG_CONFIG_HOME")
	if xdg_config_home == "" {
		xdg_config_home = os.Getenv("HOME") + "/.config"
	}
	USER_DIR = xdg_config_home
}

func setGlobalDir() {
	xdg_config_dirs := os.Getenv("XDG_CONFIG_DIRS")
	if xdg_config_dirs == "" {
		xdg_config_dirs = "/etc/xdg"
	}
	GLOBAL_DIRS = xdg_config_dirs
}

func setWorkingDir() {
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	WORKING_DIR = wd
}

func init() {
	setUserDir()
	setGlobalDir()
	setWorkingDir()
	CONFIG_EXT = ".conf"
}
