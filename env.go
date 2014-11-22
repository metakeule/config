package config

import "os"

var (
	USER_DIR    string
	GLOBAL_DIRS string // colon separated list to look for
	WORKING_DIR string
	CONFIG_EXT  string
	ENV         []string
	ARGS        []string
)

func init() {
	ENV = os.Environ()
	ARGS = os.Args[1:]
}
