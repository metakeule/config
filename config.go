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

/*
TODO make more tests
TODO make cmdline utility
TODO improve documentation
*/

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

// New creates a new *Config for the given app and version
// An error is returned, if the app and the version do not not match
// the following regular expressions:
// app => NameRegExp
// version => VersionRegexp
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

// MustNew calls New() and panics on errors
func MustNew(app string, version string) *Config {
	c, err := New(app, version)
	if err != nil {
		panic(err)
	}
	return c
}

func (c *Config) EachSpec(fn func(name string, opt *Option)) {
	for k, opt := range c.spec {
		fn(k, opt)
	}
}

func (c *Config) EachValue(fn func(name string, val interface{})) {
	for k, val := range c.values {
		fn(k, val)
	}
}

// MustSub calls Sub() and panics on errors
func (c *Config) MustSub(name string) *Config {
	s, err := c.Sub(name)
	if err != nil {
		panic(err)
	}
	return s
}

// Sub returns a *Config for a subcommand.
// If name does not match to NameRegExp, an error is returned
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

	return s, nil
}

// addOption adds the given option, validates it and returns any error
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

// Reset cleans the values, the locations and any current subcommand
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
		panic(InvalidNameError(option))
	}
	return c.locations[option]
}

// IsOption returns true, if the given option is allowed
func (c *Config) IsOption(option string) bool {
	if err := ValidateName(option); err != nil {
		return false
	}
	_, has := c.spec[option]
	return has
}

// set sets the option to the value and validates the value returning any errors
func (c *Config) set(option string, value string, location string) error {
	if err := ValidateName(option); err != nil {
		return InvalidNameError(option)
	}
	spec, has := c.spec[option]

	if !has {
		return UnknownOptionError{c.version, option}
	}

	out, err := stringToValue(spec.Type, value)

	if err != nil {
		return InvalidValueError{option, value}
	}

	c.values[option] = out
	c.locations[option] = append(c.locations[option], location)
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

// setMap sets the given options and tracks the calling function as
// location
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
		panic(InvalidNameError(option))
	}
	_, has := c.values[option]
	return has
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

// CurrentSub returns the current subcommand. It returns nil, if there is no current sub.
func (c *Config) CurrentSub() *Config {
	return c.currentSub
}

// isSub checks if the *Config relongs to a subcommand
func (c *Config) isSub() bool {
	return !(strings.Index(c.app, "_") == -1)
}

// MarshalJSON serializes the spec to JSON
func (c *Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.spec)
}

// UnmarshalJSON deserializes the spec from JSON
func (c *Config) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &c.spec)
}

// appName returns the name of the app
func (c *Config) appName() string {
	if c.isSub() {
		return c.app[:strings.Index(c.app, "_")]
	}
	return c.app
}

// subName returns the name of the subcommand and the empty string, if there is no subcommand, the empty string is returned
func (c *Config) subName() string {
	if c.isSub() {
		return c.app[strings.Index(c.app, "_")+1:]
	}
	return ""
}

// Binary returns the path to the binary of the app
func (c *Config) Binary() (path string, err error) {
	return exec.LookPath(c.appName())
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

	var keys = map[string]bool{}

	var valBuf bytes.Buffer
	var key string
	var subcommand string

	setValue := func() error {
		val := strings.TrimSpace(valBuf.String())
		if val == "" {
			if subcommand != "" {
				key = subcommand + "_" + key
			}
			return EmptyValueError(key)
		}
		// key := strings.TrimRight(key, " ")
		var err error
		if subcommand == "" {
			//fmt.Printf("setting %#v to %#v\n", key, val)
			err = c.set(key, val, location)
		} else {
			//fmt.Printf("setting %#v to %#v for subcommand %#v\n", key, val, subcommand)
			sub, has := c.subcommands[subcommand]
			if !has {
				return errors.New("unknown subcommand " + subcommand)
			} else {
				err = sub.set(key, val, location)
			}

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

	for sc.Scan() {

		pair := sc.Text()
		//fmt.Printf("pair: %#v\n", pair)

		if len(pair) == 0 {
			continue // Todo add a new line to existing values
		}

		switch pair[:1] {
		// comment
		case "#":
			continue
			// option
		case "$":
			if key != "" {
				if err := setValue(); err != nil {
					return err
				}
			}
			idx := strings.Index(pair, "=")
			if idx == -1 {
				return wrapErr(fmt.Errorf("missing '=' in %#v", pair))
			}
			key = strings.TrimRight(pair[1:idx], " ")
			if _, has := keys[key]; has {
				return ErrDoubleOption(key)
			}
			keys[key] = true
			subcommand = ""

			if underscPos := strings.Index(key, "_"); underscPos > 0 {
				subcommand = key[:underscPos]
				key = key[underscPos+1:]
			}

			// fmt.Printf("key: %#v subcommand: %#v\n", key, subcommand)

			if err := ValidateName(key); err != nil {
				return err
			}

			if subcommand != "" {
				if err := ValidateName(subcommand); err != nil {
					return err
				}
			}

			// valueMode = true
			valBuf.Reset()
			if idx < len(pair)-2 {
				valBuf.WriteString(pair[idx+1:])
			}
		default:
			valBuf.WriteString("\n" + pair)

		}

	}
	if key != "" {
		setValue()
	}
	return nil
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
				val = strings.TrimSpace(val)

				if val == "" {
					return EmptyValueError(key)
				}
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
	// prevent duplicates
	keys := map[string]bool{}
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

			if val == "" {
				err = EmptyValueError(key)
				return
			}
		} else {
			key = pair
			val = "true"
		}

		argKey := key
		key = argToKey(argKey)
		// fmt.Println(argKey)

		switch key {
		case "config-spec":
			var bt []byte
			bt, err = c.MarshalJSON()
			if err != nil {
				err = wrapErr(fmt.Errorf("can't serialize config spec to json: %#v\n", err.Error()))
				return
			}
			fmt.Fprintf(os.Stdout, "%s\n", bt)
			os.Exit(0)
		case "config-locations":
			var bt []byte
			bt, err = json.Marshal(c.locations)
			if err != nil {
				err = wrapErr(fmt.Errorf("can't serialize config locations to json: %#v\n", err.Error()))
				return
			}
			fmt.Fprintf(os.Stdout, "%s\n", bt)
			os.Exit(0)
		case "config-files":
			cfgFiles := struct {
				Global string `json:"global,omitempty"`
				User   string `json:"user,omitempty"`
				Local  string `json:"local,omitempty"`
			}{
				FirstGlobalsFile(c),
				UserFile(c),
				LocalFile(c),
			}
			var bt []byte
			bt, err = json.Marshal(cfgFiles)
			if err != nil {
				err = wrapErr(fmt.Errorf("can't serialize config files to json: %#v\n", err.Error()))
				return
			}
			fmt.Fprintf(os.Stdout, "%s\n", bt)
			os.Exit(0)
		case "version":
			fmt.Fprintf(os.Stdout, "%s\n", c.version)
			os.Exit(0)
		case "help":
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

			if keys[key] {
				err = ErrDoubleOption(key)
				return
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
			keys[key] = true
		}
	}

	if err = c.ValidateValues(); err != nil {
		return
	}
	err = c.CheckMissing()
	return
}

// GetBool returns the value of the option as bool
func (c Config) GetBool(option string) bool {
	if err := ValidateName(option); err != nil {
		panic(InvalidNameError(option))
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
		panic(InvalidNameError(option))
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
		panic(InvalidNameError(option))
	}
	v, has := c.values[option]
	if has {
		return v.(int32)
	}
	return 0
}

// GetValue returns the value of the option
func (c Config) GetValue(option string) interface{} {
	if err := ValidateName(option); err != nil {
		panic(InvalidNameError(option))
	}
	v, has := c.values[option]
	if has {
		return v
	}
	return nil
}

// GetTime returns the value of the option as time
func (c Config) GetTime(option string) *time.Time {
	if err := ValidateName(option); err != nil {
		panic(InvalidNameError(option))
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
		panic(InvalidNameError(option))
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
		panic(InvalidNameError(option))
	}
	v, has := c.values[option]
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
	if c.isSub() {
		return errors.New("WriteConfigFile must not be called in sub command")
	}
	if errValid := c.ValidateValues(); errValid != nil {
		return errValid
	}
	dir := filepath.FromSlash(filepath.Dir(path))
	info, errDir := os.Stat(dir)
	if errDir != nil {
		errDir = os.MkdirAll(dir, 0755)
		if errDir != nil {
			return errDir
		}
	} else {
		if !info.IsDir() {
			return fmt.Errorf("%s is no directory", dir)
		}
	}

	path = filepath.FromSlash(path)

	backup, errBackup := ioutil.ReadFile(path)
	backupInfo, errInfo := os.Stat(path)
	// don't write anything, if we have no config values
	if len(c.values) == 0 {
		// files exist, but will be deleted (no config values)
		if errInfo == nil {
			return os.Remove(path)
		}
		// files does not exist, we have no values, so lets do nothing
		return nil
	}
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

	// _, err = file.WriteString(c.app + " " + c.version + string(delim))
	_, err = file.WriteString(c.app + " " + c.version +
		"\n# Don't delete the first line!" +
		"\n#" +
		"\n# This is a configuration file for the command " + c.app + " of the version " + c.version + " and compatible versions." +
		"\n# All available options can be found by running" +
		"\n#" +
		"\n#           " + c.app + " --help-all" +
		"\n#" +
		"\n# ------------ FILE FORMAT ------------" +
		"\n#" +
		"\n# 1. all lines end in Unix format (LF)" +
		"\n# 2. the first line must be 'xxxx yyy' where 'xxxx' is the command name and 'yyy' is the command version" +
		"\n# 3. a line starting with '#' is a comment" +
		"\n# 4. a line starting with '$' is an option key and must have the format" +
		"\n#    '$xxxx=yyy' where 'xxxx' is the option name " +
		"\n#    and 'yyy' is the value. The '=' may be surrounded by whitespace and the value 'yyy'" +
		"\n#    may begin after a linefeed" +
		"\n# 5. the option name is like the corresponding arg without any prefixing '-'" +
		"\n#    and subcommand options are prefixed with the name of the" +
		"\n#    subcommand followed by an underscore '_'" +
		"\n# 6. Every line that does not begin with '#' or '$' is part of the value of the previous option key." +
		"\n#" +
		"\n# ------------ EXAMPLE ------------" +
		"\n#" +
		"\n#           git 2.1" +
		"\n#           # a value in the same line as the option" +
		"\n#           $commit_all=true" +
		"\n#           # a multiline value starting in the line after the option" +
		"\n#           $commit_message=" +
		"\n#           a commit message that spans" +
		"\n#           # comments are ignored" +
		"\n#           several lines" +
		"\n#           # a value in the same line as the option, = surrounded by whitespace" +
		"\n#           $commit_cleanup = verbatim" +
		"\n#" +
		"\n# The above configuration corresponds to the following command invokation (in bash):" +
		"\n#" +
		"\n#           git commit --all --cleanup=verbatim --message=$'a commit message that spans\\nseveral lines'" +
		"\n#" +
		"\n# ------------ CONFIGURATION ------------" +
		"\n#",
	)
	if err != nil {
		return
	}

	return c.writeConfigValues(file)
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

		_, err = file.WriteString("\n# --- " + writeKey + " (" + c.spec[k].Type + ") ---\n#     " + strings.Join(helplines, "\n#     ") + "\n")
		if err != nil {
			return
		}

		_, err = file.WriteString("$" + writeKey + "=")
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
			pre := ""
			if len(ty) > 15 || strings.Contains(ty, "\n") {
				pre = "\n"
			}
			_, err = file.WriteString(pre + ty)
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
			_, err = file.WriteString(" " + str)
		default:
			var bt []byte
			bt, err = json.Marshal(ty)
			if err != nil {
				return
			}
			_, err = file.WriteString("\n" + string(bt))
		}

		if err != nil {
			return
		}

		/*
			_, err = file.Write(delim)
			if err != nil {
				return
			}
		*/
	}

	for _, sub := range c.subcommands {
		_, err = file.WriteString("\n# ------------ SUBCOMMAND " + sub.subName() + " ------------\n#")
		if err != nil {
			return
		}
		sub.writeConfigValues(file)
	}
	return
}
