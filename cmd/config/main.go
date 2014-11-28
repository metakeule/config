package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	// "flag"
	// "fmt"
	// "os"
	"os/exec"

	"github.com/metakeule/config"
)

var (
	cfg               = config.MustNew("config", "0.0.1")
	optionCommand     = cfg.NewString("command", "the command where the options belong to", config.Required, config.Shortflag('c'))
	optionLocations   = cfg.NewBool("locations", "the locations where the options are currently set", config.Shortflag('l'))
	cfgSet            = cfg.MustSub("set")
	optionSetKey      = cfgSet.NewString("option", "the option that should be set", config.Required, config.Shortflag('o'))
	optionSetValue    = cfgSet.NewString("value", "the value the option should be set to", config.Required, config.Shortflag('v'))
	optionSetPathType = cfgSet.NewString("type", "the type of the config path where the value should be set", config.Shortflag('p'), config.Required)
	cfgGet            = cfg.MustSub("get")
	optionGetKey      = cfgGet.NewString("option", "the option that should be get, if not set, all options that are set are returned", config.Shortflag('o'))
	cfgPath           = cfg.MustSub("path")
	optionPathType    = cfgPath.NewString("type", "the type of the config path. valid values are global,user,local and all", config.Shortflag('p'), config.Default("all"))
)

func GetVersion(cmdpath string) (string, error) {
	cmd := exec.Command(cmdpath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// fmt.Printf("version: %#v\n", string(out))
	return strings.TrimSpace(string(out)), nil
}

func GetSpec(cmdpath string, c *config.Config) error {
	cmd := exec.Command(cmdpath, "--config-spec")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	return c.UnmarshalJSON(out)
}

func main() {
	var cmdConfig *config.Config
	var commandPath string
	var cmd string

	config.Run(cfg, "a multiplattform and multilanguage configuration tool", func(*config.Config) (err error) {
		cmd = optionCommand.Get()
		commandPath, err = exec.LookPath(cmd)
		if err != nil {
			return
		}
		var version string
		version, err = GetVersion(commandPath)
		if err != nil {
			return
		}

		cmdConfig, err = config.New(cmd, version)
		if err != nil {
			return
		}
		return GetSpec(commandPath, cmdConfig)
	})

	switch cfg.CurrentSub() {
	case nil:
		if optionLocations.IsSet() {
			err := config.Load(cmdConfig, "", false)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can't load options for command %s: %s", cmd, err.Error())
				os.Exit(1)
			}
			locations := map[string][]string{}

			cmdConfig.EachValue(func(name string, value interface{}) {
				locations[name] = cmdConfig.Locations(name)
			})

			var b []byte
			b, err = json.Marshal(locations)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can't print locations for command %s: %s", cmd, err.Error())
				os.Exit(1)
			}

			fmt.Fprintln(os.Stdout, string(b))
			os.Exit(0)
		}
		// fmt.Println("no subcommand")
	case cfgGet:
		err := config.Load(cmdConfig, "", false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Can't load config options for command %s: %s", cmd, err.Error())
			os.Exit(1)
		}
		if !optionGetKey.IsSet() {
			var vals = map[string]interface{}{}
			cmdConfig.EachValue(func(name string, value interface{}) {
				vals[name] = value
			})
			var b []byte
			b, err = json.Marshal(vals)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can't print locations for command %s: %s", cmd, err.Error())
				os.Exit(1)
			}

			fmt.Fprintln(os.Stdout, string(b))
			os.Exit(0)
		} else {
			key := optionGetKey.Get()
			if !cmdConfig.IsOption(key) {
				fmt.Fprintf(os.Stderr, "unknown option %s", cmd, err.Error())
				os.Exit(1)
			}

			val := cmdConfig.GetValue(key)
			// cmdConfig.
			fmt.Fprintf(os.Stdout, "%v\n", val)
		}

	case cfgSet:
		key := optionSetKey.Get()
		val := optionSetValue.Get()
		ty := optionSetPathType.Get()
		switch ty {
		case "user":
			if err := config.LoadUser(cmdConfig); err != nil {
				fmt.Fprintf(os.Stderr, "Can't load user config file: %s", err.Error())
				os.Exit(1)
			}
			if err := cmdConfig.Set(key, val, config.UserFile(cmdConfig)); err != nil {
				fmt.Fprintf(os.Stderr, "Can't set option %#v to value %#v: %s", key, val, err.Error())
				os.Exit(1)
			}
			if err := config.SaveToUser(cmdConfig); err != nil {
				fmt.Fprintf(os.Stderr, "Can't save user config file: %s", err.Error())
				os.Exit(1)
			}
		case "local":
			if err := config.LoadLocals(cmdConfig); err != nil {
				fmt.Fprintf(os.Stderr, "Can't load local config file: %s", err.Error())
				os.Exit(1)
			}
			if err := cmdConfig.Set(key, val, config.LocalFile(cmdConfig)); err != nil {
				fmt.Fprintf(os.Stderr, "Can't set option %#v to value %#v: %s", key, val, err.Error())
				os.Exit(1)
			}
			if err := config.SaveToLocal(cmdConfig); err != nil {
				fmt.Fprintf(os.Stderr, "Can't save local config file: %s", err.Error())
				os.Exit(1)
			}
		case "global":
			if err := config.LoadGlobals(cmdConfig); err != nil {
				fmt.Fprintf(os.Stderr, "Can't load global config file: %s", err.Error())
				os.Exit(1)
			}
			if err := cmdConfig.Set(key, val, config.FirstGlobalsFile(cmdConfig)); err != nil {
				fmt.Fprintf(os.Stderr, "Can't set option %#v to value %#v: %s", key, val, err.Error())
				os.Exit(1)
			}
			if err := config.SaveToGlobals(cmdConfig); err != nil {
				fmt.Fprintf(os.Stderr, "Can't save global config file: %s", err.Error())
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "'%s' is not a valid value for type option. possible values are 'local', 'global' or 'user'", ty)
			os.Exit(1)

		}
	case cfgPath:
		ty := optionPathType.Get()
		switch ty {
		case "user":
			fmt.Fprintln(os.Stdout, config.UserFile(cmdConfig))
			os.Exit(0)
		case "local":
			fmt.Fprintln(os.Stdout, config.LocalFile(cmdConfig))
			os.Exit(0)
		case "global":
			fmt.Fprintln(os.Stdout, config.FirstGlobalsFile(cmdConfig))
			os.Exit(0)
		case "all":
			paths := map[string]string{
				"user":   config.UserFile(cmdConfig),
				"local":  config.LocalFile(cmdConfig),
				"global": config.FirstGlobalsFile(cmdConfig),
			}
			b, err := json.Marshal(paths)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Can't print locations for command %s: %s", cmd, err.Error())
				os.Exit(1)
			}

			fmt.Fprintln(os.Stdout, string(b))
			os.Exit(0)
		default:
			fmt.Fprintf(os.Stderr, "'%s' is not a valid value for type option. possible values are 'local', 'global' or 'user'", ty)
			os.Exit(1)
		}
	// some not allowed subcommand, should already be catched by config.Run
	default:
		panic("must not happen")

	}

}

/*
tool to read and set configurations

keys consist of names that are all uppercase letters separated by underscore _

config [binary] key

returns type: value

supported types are:
bool, int32, float32, string (utf-8), datetime, json

(this reads config)

config [binary] -l key1=value1,key2=value2 // sets the options in the local config file (relative to dir)
config [binary] -u key1=value1,key2=value2 // sets the options in the user config file
config [binary] -g key1=value1,key2=value2 // sets the options in the global config file
config [binary] -c key1=value1,key2=value2 // checks the options for the binary
config [binary] -h key                     // prints help about the key
config [binary] -h                         // prints help about all options
config [binary] -m key1=value1,key2=value2 // merges the options with global/user/local ones and prints the result

each setting of an option is checked for validity of the type.
for json values it is only checked, if it is valid json. additional
checks for the json structure must be done by the binary

values are passed the following way:
boolean values: true|false
int32 values: 34523
float32 values: 4.567
string values: "here the utf-8 string"
datetime values: 2006-01-02T15:04:05Z07:00    (RFC3339)
json values: '{"a": "\'b\'"}'

a binary that is supported by config is supposed to be callable with --config-spec and then return a json encoded hash of the options in the form of
[
	{
		"key": "here_the_key1",
	  "required": true|false,
	  "type": "bool"|"int32"|"float32"|"string"|"datetime"|"json",
	  "description": "...",
	  "default": null|"value"
	},
  {
  	"key": "here_the_key2",
	  required: true|false,
	  type: "bool"|"int32"|"float32"|"string"|"datetime"|"json",
	  description: "...",
	  "default": null|"value"
	}
	[...]
]

config is meant to be used on the plattforms:
- linux
- windows
- mac os x
(maybe Android too)

it builds the configuration options by starting with defaults and merging in the following configurations
(the next overwriting the same previously defined key):

defaults as reported via [binary] --config-spec
plattform-specific global config
plattform-specific user config
local config in directory named .config/[binary] in the current working directory
environmental variables starting with [BINARY]_CONFIG_
given args to the commandline

the binary itself wants to get all options in a single go.
it therefore may run

  config [binary] -args argstring

additionally there is a library for go (and might be created for other languages)
that make it easy to query the final options in a type-safe manner

subcommands are handled as if they were extra binaries with the name
[binary]_[subcommand]: they have separate config files and if a binary name with an  underscore
is passed to config the part after the underscore is considered a subcommand.
The environment variables for subcommands start with [BINARY]_[SUBCOMMAND]_CONFIG_
when a subcommand is called the options/configuration for the binary are also loaded.


*/

/*
func main() {
	flag.Parse()
	fmt.Printf("%#v\n", os.Args)
	fmt.Printf("%#v\n", flag.Args())
}
*/
