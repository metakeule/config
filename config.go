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
	ENV         []string
	ARGS        []string
)

const (
	DateFormat     = "2006-01-02"
	TimeFormat     = "15:04:05"
	DateTimeFormat = "2006-01-02 15:04:05"
)

type Config struct {
	app       string
	version   string
	spec      map[string]*Option
	values    map[string]interface{}
	locations map[string][]string
	// maps shortflag to option
	shortflags  map[string]string
	subcommands map[string]*Config
	currentSub  *Config
}

// New creates a new *Config and returns it as ConfigGetter
func New(app string, version string) (*Config, error) {

	if err := ValidateName(app); err != nil {
		return nil, ErrInvalidAppName(app)
	}

	if err := ValidateVersion(version); err != nil {
		return nil, err
	}

	c := &Config{
		spec:        map[string]*Option{},
		subcommands: map[string]*Config{},
		app:         app,
		version:     version,
		shortflags:  map[string]string{},
	}

	c.Reset()
	return c, nil
}

// like New() but panics for invalid app names, version and options
func MustNew(app string, version string) *Config {
	c, err := New(app, version)
	if err != nil {
		panic(err)
	}
	return c
}

func (c *Config) MustSub(name string) *Config {
	s, err := c.Sub(name)
	if err != nil {
		panic(err)
	}
	return s
}

func (c *Config) Sub(name string) (*Config, error) {
	if c.isSub() {
		return nil, ErrSubSubCommand
	}
	s, err := New(name, c.version)
	if err != nil {
		return nil, err
	}

	s.app = c.app + "_" + s.app
	c.subcommands[name] = s

	/*
		for k, v := range c.spec {
			s.spec[k] = v
		}

		for k, v := range c.shortflags {
			s.shortflags[k] = v
		}
	*/

	return s, nil
}

func (c *Config) addOption(opt *Option) error {
	if err := ValidateName(opt.Name); err != nil {
		return ErrInvalidOptionName(opt.Name)
	}

	if _, has := c.spec[opt.Name]; has {
		return ErrDoubleOption(opt.Name)
	}
	c.spec[opt.Name] = opt
	if opt.Shortflag != "" {
		if _, has := c.shortflags[opt.Shortflag]; has {
			return ErrDoubleShortflag(opt.Shortflag)
		}
		c.shortflags[opt.Shortflag] = opt.Name
	}
	return nil
}

// Reset cleans the values
func (c *Config) Reset() {
	c.values = map[string]interface{}{}
	c.locations = map[string][]string{}
	c.currentSub = nil
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
	if err := ValidateName(option); err != nil {
		panic(ErrInvalidName)
	}
	return c.locations[option]
}

func (c *Config) set(key string, val string, location string) error {
	if err := ValidateName(key); err != nil {
		return ErrInvalidName
	}
	spec, has := c.spec[key]

	if !has {
		return UnknownOptionError{c.version, key}
	}

	out, err := stringToValue(spec.Type, val)

	if err != nil {
		return InvalidValueError{key, val}
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
func (c Config) IsSet(option string) bool {
	if err := ValidateName(option); err != nil {
		panic(ErrInvalidName)
	}
	_, has := c.values[option]
	return has
}

// GetBool returns the value of the option as bool
func (c Config) GetBool(option string) bool {
	if err := ValidateName(option); err != nil {
		panic(ErrInvalidName)
	}
	v, has := c.values[option]
	if has {
		return v.(bool)
	}
	return false
}

// GetFloat32 returns the value of the option as float32
func (c Config) GetFloat32(option string) float32 {
	if err := ValidateName(option); err != nil {
		panic(ErrInvalidName)
	}
	v, has := c.values[option]
	if has {
		return v.(float32)
	}
	return 0
}

// GetInt32 returns the value of the option as int32
func (c Config) GetInt32(option string) int32 {
	if err := ValidateName(option); err != nil {
		panic(ErrInvalidName)
	}
	v, has := c.values[option]
	if has {
		return v.(int32)
	}
	return 0
}

// GetTime returns the value of the option as time
func (c Config) GetTime(option string) *time.Time {
	if err := ValidateName(option); err != nil {
		panic(ErrInvalidName)
	}
	v, has := c.values[option]
	if has {
		val := v.(time.Time)
		return &val
	}
	return nil
}

// GetString returns the value of the option as string
func (c Config) GetString(option string) string {
	if err := ValidateName(option); err != nil {
		panic(ErrInvalidName)
	}
	v, has := c.values[option]
	if has {
		return v.(string)
	}
	return ""
}

// GetJSON unmarshals the value of the option to val.
func (c Config) GetJSON(option string, val interface{}) error {
	if err := ValidateName(option); err != nil {
		panic(ErrInvalidName)
	}
	v, has := c.values[option]
	if has {
		return json.Unmarshal([]byte(v.(string)), val)
	}
	return nil
}

func (c *Config) writeConfigValues(file *os.File) (err error) {

	for k, v := range c.values {
		// do nothing for nil values
		if v == nil {
			continue
		}

		help := strings.Split(c.spec[k].Help, "\n")
		helplines := []string{}

		for _, h := range help {
			helplines = append(helplines, strings.TrimSpace(h))
		}

		writeKey := k
		if c.isSub() {
			writeKey = c.subName() + "_" + k
		}

		_, err = file.WriteString("# ------ " + writeKey + " (" + c.spec[k].Type + ") ------\n# " + strings.Join(helplines, "\n# ") + "\n")
		if err != nil {
			return
		}

		_, err = file.WriteString(writeKey + "=")
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
			var str string
			switch c.spec[k].Type {
			case "date":
				str = ty.Format(DateFormat)
			case "time":
				str = ty.Format(TimeFormat)
			case "datetime":
				str = ty.Format(DateTimeFormat)
			default:
				return InvalidTypeError{k, c.spec[k].Type}
				// return ErrInvalidType(c.spec[k].Type)
			}
			_, err = file.WriteString(str)
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

	for _, sub := range c.subcommands {
		_, err = file.WriteString("#### " + sub.subName() + " ####\n")
		if err != nil {
			return
		}
		sub.writeConfigValues(file)
	}
	return
}

// WriteConfigFile writes the configuration values to the given file
// The file is overwritten/created on success and a backup of an existing file is written back
// if an error happens
// the given perm is only used to create new files.
func (c *Config) WriteConfigFile(path string, perm os.FileMode) (err error) {
	if c.isSub() {
		return errors.New("WriteConfigFile must not be called in sub command")
	}
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

	return c.writeConfigValues(file)
}

func (c *Config) Merge(rd io.Reader, location string) error {
	wrapErr := func(err error) error {
		return InvalidConfigFileError{location, c.version, err}
	}

	sc := bufio.NewScanner(rd)
	if !sc.Scan() {
		return wrapErr(errors.New("can't read config header (app and version)"))
	}
	header := sc.Text()
	words := strings.Split(header, " ")
	if len(words) != 2 {
		return wrapErr(errors.New("invalid config header"))
	}
	if words[0] != c.appName() {
		return wrapErr(fmt.Errorf("invalid config header: app is %#v but config is for app %#v", c.appName(), words[0]))
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
			return 0, nil, wrapErr(fmt.Errorf("invalid data: %#v", string(data)))
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

		// ignore comments to the end of line
		for strings.HasPrefix(pair, "#") {
			idx := strings.Index(pair, "\n")
			if idx == -1 || len(pair) < idx+1 {
				return wrapErr(fmt.Errorf("comment must end with newline"))
			}
			pair = pair[idx+1:]
		}
		/*
			if strings.HasPrefix(pair, "#") {

			}
		*/
		// fmt.Printf("text: %#v\n", pair)
		ass := strings.Index(pair, "=")
		if ass == -1 {
			return wrapErr(fmt.Errorf("missing = in %#v", pair))
		}
		key, val := pair[:ass], pair[ass+1:]

		underscPos := strings.Index(key, "_")

		var err error
		if underscPos == -1 {
			err = c.set(key, val, location)
		} else {
			subName := key[:underscPos]
			sub, has := c.subcommands[subName]
			if !has {
				// fmt.Printf("subcommands: %#v (app: %#v)\n", c.subcommands, c.app)
				return errors.New("unknown subcommand " + subName)
			} else {
				err = sub.set(key[underscPos+1:], val, location)
			}
		}

		// key = strings.TrimLeft(key, "\n")
		if err != nil {
			if differentVersions {
				return wrapErr(fmt.Errorf("value %#v of option %s, present in config for version %s is not valid for running version %s",
					val, key, words[1], c.version))
			} else {
				return wrapErr(err)
			}
		}
	}
	return nil
}

func init() {
	ENV = os.Environ()
	ARGS = os.Args[1:]
}

func (c *Config) MergeEnv() error {
	prefix := strings.ToUpper(c.app) + "_CONFIG_"
	// fmt.Printf("looking for prefix %#v\n", prefix)
	for _, pair := range ENV {
		if strings.HasPrefix(pair, prefix) {
			// fmt.Printf("Env: %#v\n", pair)
			startKey := len(prefix) // strings.Index(pair, prefix)
			if startKey > 0 {
				startVal := strings.Index(pair, "=")
				key, val := pair[startKey:startVal], pair[startVal+1:]
				// fmt.Printf("key %#v val %#v\n", key, val)
				err := c.set(strings.ToLower(key), val, pair[:startVal])
				if err != nil {
					return InvalidConfigEnv{c.version, pair[:startVal], err}
				}
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
func (c *Config) MergeArgs(helpIntro string) error {
	_, err := c.mergeArgs(helpIntro, false, ARGS)
	return err
}

func (c *Config) mergeArgs(helpIntro string, ignoreUnknown bool, args []string) (merged map[string]bool, err error) {
	merged = map[string]bool{}
	// fmt.Printf("args: %#v\n", os.Args[1:])
	for _, pair := range args {
		wrapErr := func(err error) error {
			return InvalidConfigFlag{c.version, pair, err}
		}
		idx := strings.Index(pair, "=")
		var key, val string
		if idx != -1 {
			if !(idx < len(pair)-1) {
				err = wrapErr(fmt.Errorf("invalid argument syntax at %#v\n", pair))
				return
			}
			key, val = pair[:idx], pair[idx+1:]
		} else {
			key = pair
			val = "true"
		}

		argKey := key
		key = argToKey(argKey)
		// fmt.Println(argKey)

		switch key {
		case "CONFIG_SPEC":
			var bt []byte
			bt, err = c.MarshalJSON()
			if err != nil {
				err = wrapErr(fmt.Errorf("can't serialize config spec to json: %#v\n", err.Error()))
				return
			}
			fmt.Fprintf(os.Stdout, "%s\n", bt)
			os.Exit(0)
		case "CONFIG_LOCATIONS":
			var bt []byte
			bt, err = json.Marshal(c.locations)
			if err != nil {
				err = wrapErr(fmt.Errorf("can't serialize config locations to json: %#v\n", err.Error()))
				return
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
			var bt []byte
			bt, err = json.Marshal(cfgFiles)
			if err != nil {
				err = wrapErr(fmt.Errorf("can't serialize config files to json: %#v\n", err.Error()))
				return
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

			// fmt.Println(key)
			_, has := c.spec[key]
			if ignoreUnknown && !has {
				continue
			}
			err = c.set(key, val, argKey)
			if err != nil {
				err = wrapErr(fmt.Errorf("invalid value for option %s: %s\n", key, err.Error()))
				return
			}
			merged[argKey] = true
		}
	}

	if err = c.ValidateValues(); err != nil {
		return
	}
	err = c.CheckMissing()
	return
}

// CheckMissing checks if mandatory values are missing inside the values map
// CheckMissing stops on the first error
func (c *Config) CheckMissing() error {
	for k, spec := range c.spec {
		if spec.Required && spec.Default == nil {
			if _, has := c.values[k]; !has {
				return MissingOptionError{c.version, k}
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
			return UnknownOptionError{c.version, k}
			// return errors.New("unkown config key " + k)
		}
		if err := spec.ValidateValue(v); err != nil {
			return InvalidConfig{c.version, err}
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
		err, found := c.LoadFile(filepath.Join(dir, c.appName(), c.appName()+CONFIG_EXT))
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
	return filepath.Join(dir, c.appName(), c.appName()+CONFIG_EXT)
}

// GlobalFile returns the path for the global config file in the first global directory
func (c *Config) FirstGlobalsFile() string {
	return c.globalsFile(strings.Split(GLOBAL_DIRS, ":")[0])
}

func (c *Config) UserFile() string {
	return filepath.Join(USER_DIR, c.appName(), c.appName()+CONFIG_EXT)
}

func (c *Config) LoadUser() error {
	err, found := c.LoadFile(c.UserFile())
	if found {
		return err
	}
	return nil
}

func (c *Config) LocalFile() string {
	return filepath.Join(WORKING_DIR, ".config", c.appName(), c.appName()+CONFIG_EXT)
}

// LoadLocals merges config inside a .config subdir in the local directory
func (c *Config) LoadLocals() error {
	// fmt.Println("loading locals from " + c.LocalFile())
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
func (c *Config) Run(helpIntro string, validator func(*Config) error) {
	err2Stderr(c.Load(helpIntro))
	if validator != nil {
		err2Stderr(validator(c))
	}
}

// returns nil, if there is no current sub
func (c *Config) CurrentSub() *Config {
	return c.currentSub
}

func (c *Config) isSub() bool {
	return !(strings.Index(c.app, "_") == -1)
}

func (c *Config) appName() string {
	if c.isSub() {
		return c.app[:strings.Index(c.app, "_")]
	}
	return c.app
}

func (c *Config) subName() string {
	if c.isSub() {
		return c.app[strings.Index(c.app, "_")+1:]
	}
	return ""
}

func (c *Config) Load(helpIntro string) error {
	// clear old values
	c.Reset()

	// fmt.Printf("ARGS: %#v\n", ARGS)

	// first load defaults
	c.LoadDefaults()

	// then overwrite with globals, return any error
	if err := c.LoadGlobals(); err != nil {
		return err
	}

	// then overwrite with user, return any error
	if err := c.LoadUser(); err != nil {
		return err
	}

	// then overwrite with locals, return any error
	if err := c.LoadLocals(); err != nil {
		return err
	}

	// then overwrite with env, return any error
	if err := c.MergeEnv(); err != nil {
		return err
	}

	if len(ARGS) > 0 {
		// fmt.Println("we are in subcommand " + ARGS[0])
		if sub, has := c.subcommands[strings.ToLower(ARGS[0])]; has {
			// fmt.Println("we are in subcommand " + ARGS[0])
			c.currentSub = sub
			if len(ARGS) == 1 {
				ARGS = []string{}
			} else {
				ARGS = ARGS[1:]
			}

			// then overwrite with env, return any error
			if err := sub.MergeEnv(); err != nil {
				return err
			}

			merged1, err1 := c.mergeArgs(helpIntro, true, ARGS)
			if err1 != nil {
				return err1
			}

			// then overwrite with args
			merged2, err2 := sub.mergeArgs(helpIntro, true, ARGS)
			if err2 != nil {
				return err2
			}

			// fmt.Printf("merged1: %#v\nmerged2: %#v\n", merged1, merged2)

			for _, arg := range ARGS {
				key := arg
				if idx := strings.Index(arg, "="); idx != -1 {
					key = arg[:idx]
				}

				if !merged1[key] && !merged2[key] {
					return UnknownOptionError{c.version, arg}
				}
			}
			return nil

			//return sub.Load(helpIntro)
		}
	}

	// then overwrite with args
	return c.MergeArgs(helpIntro)
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
