package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	word_regexp     = regexp.MustCompile("^[A-Z][A-Z0-9]+$")
	VersionRegexp   = regexp.MustCompile("^[a-z0-9-.]+$")
	ShortflagRegexp = regexp.MustCompile("^[a-z]$")
)

func ValidateShortflag(shortflag string) error {
	if shortflag == "" || ShortflagRegexp.MatchString(shortflag) {
		return nil
	}
	return ErrInvalidShortflag
}

// ValidateName checks if the given name conforms to the
// naming convention. If it does, nil is returned, otherwise
// ErrInvalidName is returned
func ValidateName(name string) error {
	if name == "" {
		return ErrInvalidName
	}
	for _, word := range strings.Split(name, "_") {
		// fmt.Printf("%#v\n", word)
		if !word_regexp.MatchString(word) {
			return ErrInvalidName
		}
	}
	return nil
}

func ValidateVersion(version string) error {
	if !VersionRegexp.MatchString(version) {
		return ErrInvalidVersion
	}
	return nil
}

// ValidateType checks if the given type is valid.
// If it does, nil is returned, otherwise
// ErrInvalidType is returned
func ValidateType(typ string) error {
	switch typ {
	case "bool", "int32", "float32", "string", "datetime", "json":
		return nil
	default:
		return ErrInvalidType
	}
}

var delim = []byte("\u220e\n")

func stringToValue(typ string, in string) (out interface{}, err error) {
	switch typ {
	case "bool":
		return strconv.ParseBool(in)
	case "int32":
		i, e := strconv.ParseInt(in, 10, 32)
		return int32(i), e
	case "float32":
		fl, e := strconv.ParseFloat(in, 32)
		return float32(fl), e
	case "datetime":
		return time.Parse(time.RFC3339, in)
	case "string":
		return in, nil
	case "json":
		var v interface{}
		err = json.Unmarshal([]byte(in), &v)
		if err != nil {
			return nil, err
		}
		return in, nil
	default:
		return nil, errors.New("unknown type " + typ)
	}

}

func keyToArg(key string) string {
	out := strings.Replace(key, "_", "-", -1)
	out = strings.ToLower(out)
	return "--" + out
}

func argToKey(arg string) string {
	out := strings.TrimLeft(arg, "-")
	out = strings.TrimLeft(out, "-")
	out = strings.ToUpper(out)
	return strings.Replace(out, "-", "_", -1)
}

func err2Stderr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}
