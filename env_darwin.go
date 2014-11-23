// +build darwin
package config

/*
according to http://wiki.freepascal.org/Multiplatform_Programming_Guide#Configuration_files
/etc
/Users/user/.config/project1
*/

import "os"

func setUserDir() {
	USER_DIR = os.Getenv("HOME") + "/.config"
}

func setGlobalDir() {
	GLOBAL_DIRS = "/etc/config"
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
