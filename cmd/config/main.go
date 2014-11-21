package main

import (
	"flag"
	"fmt"
	"os"
)

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

func main() {
	flag.Parse()
	fmt.Printf("%#v\n", os.Args)
	fmt.Printf("%#v\n", flag.Args())
}
