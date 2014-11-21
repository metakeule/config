// +build windows

// set USER_DIR, GLOBAL_DIRS and WORKING_DIR based on the environment variables
// see http://ss64.com/nt/syntax-variables.html
package config

import (
	"os"
	"path/filepath"
)

func setUserDir() {
	user_app_data := os.Getenv("LOCALAPPDATA")
	if user_app_data == "" {
		user_app_data = filepath.Join(os.Getenv("HOMEPATH"), "AppData", "Local")
	}
	USER_DIR = user_app_data
}

func setGlobalDir() {
	programData := os.Getenv("ALLUSERSPROFILE")
	if programData == "" {
		programData = os.Getenv("ProgramData")
	}
	GLOBAL_DIRS = programData
}

func setWorkingDir() {
	wd, err := os.Getwd()
	if err != nil {
		wd = os.Getenv("CD")
	}

	WORKING_DIR = wd
}

func init() {
	setUserDir()
	setGlobalDir()
	setWorkingDir()
	CONFIG_EXT = ".cfg"
}
