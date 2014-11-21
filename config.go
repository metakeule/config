package config

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var (
	USER_DIR    string
	GLOBAL_DIRS string // colon separated list to look for
	WORKING_DIR string
	CONFIG_EXT  string
)

// TODO always return the app version for invalid keys and values
// TODO add subcommands
// TODO improve and streamline errors

type ConfigGetter interface {
	Load(helpIntro string)
	IsSet(option string) bool
	Locations(option string) []string
	GetBool(option string) bool
	GetFloat32(option string) float32
	GetInt32(option string) int32
	GetTime(option string) *time.Time
	GetString(option string) string
	GetJSON(option string, val interface{}) error
}

type ConfigSetter interface {
	SetGlobalOptions(opts map[string]string) error
	SetUserOptions(opts map[string]string) error
	SetLocalOptions(opts map[string]string) error
}

var _ ConfigSetter = &Config{}

type Config struct {
	app       string
	version   string
	spec      map[string]*Option
	values    map[string]interface{}
	locations map[string][]string
	// maps shortflag to option
	shortflags map[string]string
}

// New creates a new *Config and returns it as ConfigGetter
// It panics for invalid app names, version and options
func New(app string, version string) *Config {

	if err := ValidateName(strings.ToUpper(app)); err != nil {
		panic(ErrInvalidAppName(app))
	}

	if err := ValidateVersion(version); err != nil {
		panic(err)
	}

	c := &Config{
		spec:       map[string]*Option{},
		app:        app,
		version:    version,
		shortflags: map[string]string{},
	}

	c.Reset()
	/*
		for _, opt := range options {
			c.addOption(opt)
		}
	*/

	return c
}

func (c *Config) addOption(opt *Option) {
	if err := ValidateName(opt.Name); err != nil {
		panic(ErrInvalidOptionName(opt.Name))
	}

	if _, has := c.spec[opt.Name]; has {
		panic(ErrDoubleOption(opt.Name))
	}
	c.spec[opt.Name] = opt
	if opt.Shortflag != "" {
		if _, has := c.shortflags[opt.Shortflag]; has {
			panic(ErrDoubleShortflag(opt.Shortflag))
		}
		c.shortflags[opt.Shortflag] = opt.Name
	}
}

// Reset cleans the values
func (c *Config) Reset() {
	c.values = map[string]interface{}{}
	c.locations = map[string][]string{}
}

// Location returns the locations where the option was set in the order of setting.
//
// The locations are tracked differently:
// - defaults are tracked by their %v printed value
// - environment variables are tracked by their name
// - config files are tracked by their path
// - cli args are tracked by their name
// - settings via Set() are tracked by the given location or the caller if that is empty
func (c *Config) Locations(option string) []string {
	return c.locations[strings.ToUpper(option)]
}

func (c *Config) set(key string, val string, location string) error {
	key = strings.ToUpper(key)
	spec, has := c.spec[key]

	if !has {
		return ErrUnknownOption(key)
	}

	out, err := stringToValue(spec.Type, val)

	if err != nil {
		return err
	}

	c.values[key] = out
	c.locations[key] = append(c.locations[key], location)
	return nil
}

// Set sets the option to the value. Location is a hint from where the
// option setting was triggered. If the location is empty, the caller file
// and line is tracked as location.
func (c *Config) Set(option string, val string, location string) error {
	if location == "" {
		_, file, line, _ := runtime.Caller(0)
		location = fmt.Sprintf("%s:%d", file, line)
	}
	return c.set(option, val, location)
}

// setMap sets the given options.
func (c *Config) setMap(options map[string]string) error {
	_, file, line, _ := runtime.Caller(1)
	location := fmt.Sprintf("%s:%d", file, line)

	for opt, val := range options {
		err := c.set(opt, val, location)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsSet returns true, if the given option is set and false if not.
func (c Config) IsSet(key string) bool {
	_, has := c.values[strings.ToUpper(key)]
	return has
}

// GetBool returns the value of the option as bool
func (c Config) GetBool(option string) bool {
	v, has := c.values[strings.ToUpper(option)]
	if has {
		return v.(bool)
	}
	return false
}

// GetFloat32 returns the value of the option as float32
func (c Config) GetFloat32(option string) float32 {
	v, has := c.values[strings.ToUpper(option)]
	if has {
		return v.(float32)
	}
	return 0
}

// GetInt32 returns the value of the option as int32
func (c Config) GetInt32(option string) int32 {
	v, has := c.values[strings.ToUpper(option)]
	if has {
		return v.(int32)
	}
	return 0
}

// GetTime returns the value of the option as time
func (c Config) GetTime(option string) *time.Time {
	v, has := c.values[strings.ToUpper(option)]
	if has {
		val := v.(time.Time)
		return &val
	}
	return nil
}

// GetString returns the value of the option as string
func (c Config) GetString(option string) string {
	v, has := c.values[strings.ToUpper(option)]
	if has {
		return v.(string)
	}
	return ""
}

// GetJSON unmarshals the value of the option to val.
func (c Config) GetJSON(option string, val interface{}) error {
	v, has := c.values[strings.ToUpper(option)]
	if has {
		return json.Unmarshal([]byte(v.(string)), val)
	}
	return nil
}

// WriteConfigFile writes the configuration values to the given file
// The file is overwritten/created on success and a backup of an existing file is written back
// if an error happens
// the given perm is only used to create new files.
func (c *Config) WriteConfigFile(path string, perm os.FileMode) (err error) {
	if errValid := c.ValidateValues(); errValid != nil {
		return errValid
	}
	dir := filepath.Dir(path)
	info, errDir := os.Stat(dir)
	if errDir != nil {
		errDir = os.MkdirAll(dir, 0755)
		if errDir != nil {
			return errDir
		}
	} else {
		if !info.IsDir() {
			return fmt.Errorf("%s is no directory", filepath.Dir(path))
		}
	}

	backup, errBackup := ioutil.ReadFile(path)
	backupInfo, errInfo := os.Stat(path)
	if errBackup != nil {
		backup = []byte{}
	}
	if errInfo == nil {
		perm = backupInfo.Mode()
	}
	file, errCreate := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if errCreate != nil {
		return errCreate
	}

	defer func() {
		file.Close()
		if err != nil {
			os.Remove(path)
			if len(backup) != 0 {
				ioutil.WriteFile(path, backup, perm)
			}
		}
	}()

	_, err = file.WriteString(c.app + " " + c.version + string(delim))
	if err != nil {
		return
	}

	for k, v := range c.values {
		// do nothing for nil values
		if v == nil {
			continue
		}

		_, err = file.WriteString(k + "=")
		if err != nil {
			return
		}

		switch ty := v.(type) {
		case bool:
			_, err = file.WriteString(fmt.Sprintf("%v", ty))
		case int32:
			_, err = file.WriteString(fmt.Sprintf("%v", ty))
		case float32:
			_, err = file.WriteString(fmt.Sprintf("%v", ty))
		case string:
			_, err = file.WriteString(ty)
		case time.Time:
			_, err = file.WriteString(ty.Format(time.RFC3339))
		default:
			var bt []byte
			bt, err = json.Marshal(ty)
			if err != nil {
				return
			}
			_, err = file.Write(bt)
		}

		if err != nil {
			return
		}

		_, err = file.Write(delim)
		if err != nil {
			return
		}
	}
	return
}

func (c *Config) Merge(rd io.Reader, location string) error {
	sc := bufio.NewScanner(rd)
	if !sc.Scan() {
		return errors.New("can't read config header (app and version)")
	}
	header := sc.Text()
	words := strings.Split(header, " ")
	if len(words) != 2 {
		return errors.New("invalid config header")
	}
	if words[0] != c.app {
		return fmt.Errorf("invalid config header: app is %#v but config is for app %#v", c.app, words[0])
	}

	differentVersions := words[1] != c.version

	sc.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF {
			// fmt.Printf("at EOF with data: %s\n", string(data))
			return len(data), data, io.EOF
		}
		idx := bytes.Index(data, delim)
		if idx == -1 {
			// fmt.Printf("XX invalid data: %#v\n", string(data))
			return 0, nil, errors.New("invalid data")
		}
		return idx + len(delim), data[:idx], nil
	})

	for sc.Scan() {
		pair := sc.Text()
		// skip empty lines
		if pair == "\n" {
			// return errors.New("empty pair")
			continue
		}
		if pair == "" {
			// return errors.New("empty pair")
			continue
		}
		// fmt.Printf("text: %#v\n", pair)
		ass := strings.Index(pair, "=")
		if ass == -1 {
			return errors.New("missing =")
		}
		key, val := pair[:ass], pair[ass+1:]
		// key = strings.TrimLeft(key, "\n")
		err := c.set(key, val, location)
		if err != nil {
			if differentVersions {
				return fmt.Errorf("value %#v of option %s, present in config for version %s is not valid for running version %s",
					val, key, words[1], c.version)
			} else {
				return err
			}
		}
	}
	return nil
}

func (c *Config) MergeEnv() error {
	prefix := strings.ToUpper(c.app) + "_CONFIG_"
	// fmt.Printf("looking for prefix %#v\n", prefix)
	for _, pair := range os.Environ() {
		if strings.HasPrefix(pair, prefix) {
			// fmt.Printf("Env: %#v\n", pair)
			startKey := len(prefix) // strings.Index(pair, prefix)
			startVal := strings.Index(pair, "=")
			key, val := pair[startKey:startVal], pair[startVal+1:]
			// fmt.Printf("key %#v val %#v\n", key, val)
			err := c.set(key, val, pair[:startVal])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// MergeArgs merges the os.Args into the config
// args like --a-key='a val' will correspond to the config value
// A_KEY=a val
// If the key is CONFIG_SPEC, MergeArgs will print the config spec as json
// and exit the program
// If any error happens the error will be printed to os.StdErr and the program exists will
// status code 1
// exiting the program. also if --config_spec is set the spec is directly written to the
// StdOut and the program is exiting. If --help is set, the help message is printed with the
// the help  messages for the config options. If --version is set, the version of the running app is returned
func (c *Config) MergeArgs(helpIntro string) {
	// fmt.Printf("args: %#v\n", os.Args[1:])
	for _, pair := range os.Args[1:] {
		idx := strings.Index(pair, "=")
		var key, val string
		if idx != -1 {
			if !(idx < len(pair)-1) {
				err2Stderr(fmt.Errorf("invalid argument syntax at %#v\n", pair))
			}
			key, val = pair[:idx], pair[idx+1:]
		} else {
			key = pair
			val = "true"
		}

		argKey := key
		key = argToKey(argKey)

		switch key {
		case "CONFIG_SPEC":
			bt, err := c.MarshalJSON()
			if err != nil {
				err2Stderr(fmt.Errorf("can't serialize config spec to json: %#v\n", err.Error()))
			}
			fmt.Fprintf(os.Stdout, "%s\n", bt)
			os.Exit(0)
		case "CONFIG_LOCATIONS":
			bt, err := json.Marshal(c.locations)
			if err != nil {
				err2Stderr(fmt.Errorf("can't serialize config locations to json: %#v\n", err.Error()))
			}
			fmt.Fprintf(os.Stdout, "%s\n", bt)
			os.Exit(0)
		case "CONFIG_FILES":
			cfgFiles := struct {
				Global string `json:"global,omitempty"`
				User   string `json:"user,omitempty"`
				Local  string `json:"local,omitempty"`
			}{
				c.FirstGlobalsFile(),
				c.UserFile(),
				c.LocalFile(),
			}
			bt, err := json.Marshal(cfgFiles)
			if err != nil {
				err2Stderr(fmt.Errorf("can't serialize config files to json: %#v\n", err.Error()))
			}
			fmt.Fprintf(os.Stdout, "%s\n", bt)
			os.Exit(0)
		case "VERSION":
			fmt.Fprintf(os.Stdout, "%s\n", c.version)
			os.Exit(0)
		case "HELP":
			fmt.Fprintf(os.Stdout, "%s\n", helpIntro)

			for k, spec := range c.spec {
				k = keyToArg(k)
				fmt.Fprintf(
					os.Stdout, "%s\n\t%s\n",
					k, strings.Join(strings.Split(spec.Help, "\n"), "\n\t"),
				)
			}
			os.Exit(0)
		default:
			if sh, has := c.shortflags[key]; has {
				key = sh
			}
			err := c.set(key, val, argKey)
			if err != nil {
				err2Stderr(fmt.Errorf("invalid value for option %s: %s\n", key, err.Error()))
			}
		}
	}

	err2Stderr(c.CheckMissing())
}

// CheckMissing checks if mandatory values are missing inside the values map
// CheckMissing stops on the first error
func (c *Config) CheckMissing() error {
	for k, spec := range c.spec {
		if spec.Required && spec.Default == nil {
			if _, has := c.values[k]; !has {
				return fmt.Errorf("missing config key/arg %s/%s", k, keyToArg(k))
			}
		}
	}
	return nil
}

// ValidateValues validates only values that are set and not nil.
// It does not check for missing mandatory values (use CheckMissing for that)
// ValidateValues stops on the first error
func (c *Config) ValidateValues() error {
	for k, v := range c.values {
		if v == nil {
			continue
		}
		spec, has := c.spec[k]
		if !has {
			return errors.New("unkown config key " + k)
		}
		if err := spec.ValidateValue(v); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) LoadDefaults() {
	for k, spec := range c.spec {
		if spec.Default != nil {
			c.values[k] = spec.Default
			c.locations[k] = append(c.locations[k], fmt.Sprintf("%v", spec.Default))
		}
	}
}

// LoadFile merges the config from the given file and returns any error happening during the merge
// If the file could not be opened (does not exist), no error is returned
// TODO maybe an error should be returned, if the file exists, but could not be opened because
// of missing access rights
func (c *Config) LoadFile(path string) (err error, found bool) {
	file, err0 := os.Open(path)
	if err0 != nil {
		return nil, false
	}
	found = true
	defer file.Close()
	err1 := c.Merge(file, path)
	if err1 != nil {
		err = fmt.Errorf("can't merge file %s: %s", file.Name(), err1.Error())
	}
	return
}

// LoadGlobals loads the first config file for the app it could find inside
// the GLOBAL_DIRS and returns an error if the config could not be merged properly
// If no config file could be found, no error is returned.
func (c *Config) LoadGlobals() error {
	for _, dir := range strings.Split(GLOBAL_DIRS, ":") {
		err, found := c.LoadFile(filepath.Join(dir, c.app, c.app+CONFIG_EXT))
		if found {
			return err
		}
	}
	return nil
}

func (c *Config) SetGlobalOptions(options map[string]string) error {
	c.Reset()
	if err := c.setMap(options); err != nil {
		return err
	}
	return c.SaveToGlobals()
}

func (c *Config) SetUserOptions(options map[string]string) error {
	c.Reset()
	if err := c.setMap(options); err != nil {
		return err
	}
	return c.SaveToUser()
}

func (c *Config) SetLocalOptions(options map[string]string) error {
	c.Reset()
	if err := c.setMap(options); err != nil {
		return err
	}
	return c.SaveToLocal()
}

// SaveToGlobals saves the given config values to a global config file
// don't save secrets inside the global config, since it is readable for everyone
// A new global config is written with 0644. The config is saved inside the first
// directory of GLOBAL_DIRS
func (c *Config) SaveToGlobals() error {
	if GLOBAL_DIRS == "" {
		return errors.New("GLOBAL_DIRS not set")
	}
	return c.WriteConfigFile(c.FirstGlobalsFile(), 0644)
}

// SaveToUser saves all values to the user config file
// creating missing directories
// A new config is written with 0640, ro readable for user group and writeable for the user
func (c *Config) SaveToUser() error {
	if USER_DIR == "" {
		return errors.New("USER_DIR not set")
	}
	return c.WriteConfigFile(c.UserFile(), 0640)
}

// SaveToLocal saves all values to the local config file
// A new config is written with 0640, ro readable for user group and writeable for the user
func (c *Config) SaveToLocal() error {
	if WORKING_DIR == "" {
		return errors.New("WORKING_DIR not set")
	}
	return c.WriteConfigFile(c.LocalFile(), 0640)
}

func (c *Config) globalsFile(dir string) string {
	return filepath.Join(dir, c.app, c.app+CONFIG_EXT)
}

// GlobalFile returns the path for the global config file in the first global directory
func (c *Config) FirstGlobalsFile() string {
	return c.globalsFile(strings.Split(GLOBAL_DIRS, ":")[0])
}

func (c *Config) UserFile() string {
	return filepath.Join(USER_DIR, c.app, c.app+CONFIG_EXT)
}

func (c *Config) LoadUser() error {
	err, found := c.LoadFile(c.UserFile())
	if found {
		return err
	}
	return nil
}

func (c *Config) LocalFile() string {
	return filepath.Join(WORKING_DIR, ".config", c.app, c.app+CONFIG_EXT)
}

// LoadLocals merges config inside a .config subdir in the local directory
func (c *Config) LoadLocals() error {
	err, found := c.LoadFile(c.LocalFile())
	if found {
		return err
	}
	return nil
}

// Load loads the config values in the following order where
// each loader overwrittes corresponding config keys that have been defined
/*
	defaults
	global config
	user config
	local config
	env config
	args config
*/
// in the args config any wrong syntax or values result in writing the error to StdErr and
// exiting the program. also if --config_spec is set the spec is directly written to the
// StdOut and the program is exiting. If --help is set, the help message is printed with the
// the help  messages for the config options
func (c *Config) Load(helpIntro string) {
	// clear old values
	c.Reset()

	// first load defaults
	c.LoadDefaults()

	// then overwrite with globals, return any error
	err2Stderr(c.LoadGlobals())

	// then overwrite with user, return any error
	err2Stderr(c.LoadUser())

	// then overwrite with locals, return any error
	err2Stderr(c.LoadLocals())

	// then overwrite with env, return any error
	err2Stderr(c.MergeEnv())

	// then overwrite with args
	c.MergeArgs(helpIntro)
}

func (c *Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.spec)
}

func (c *Config) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &c.spec)
}

func (c *Config) GetSpecFromBinary() error {
	p, err := exec.LookPath(c.app)
	if err != nil {
		return err
	}

	cmd := exec.Command(p, "--config_spec")
	var out []byte
	out, err = cmd.Output()
	if err != nil {
		return err
	}
	return c.UnmarshalJSON(out)
}
