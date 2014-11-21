package config

import (
	"encoding/json"
	"strings"
	"time"
)

// panics for invalid values
func (c *Config) NewBool(name, helpText string, setter ...func(*Option)) *BoolGetter {
	return &BoolGetter{
		opt: c.mkOption(name, "bool", helpText, setter),
		cfg: c,
	}
}

// panics for invalid values
func (c *Config) NewInt32(name, helpText string, setter ...func(*Option)) *Int32Getter {
	return &Int32Getter{
		opt: c.mkOption(name, "int32", helpText, setter),
		cfg: c,
	}
}

// panics for invalid values
func (c *Config) NewFloat32(name, helpText string, setter ...func(*Option)) *Float32Getter {
	return &Float32Getter{
		opt: c.mkOption(name, "float32", helpText, setter),
		cfg: c,
	}
}

// panics for invalid values
func (c *Config) NewString(name, helpText string, setter ...func(*Option)) *StringGetter {
	return &StringGetter{
		opt: c.mkOption(name, "string", helpText, setter),
		cfg: c,
	}
}

// panics for invalid values
func (c *Config) NewDateTime(name, helpText string, setter ...func(*Option)) *DateTimeGetter {
	return &DateTimeGetter{
		opt: c.mkOption(name, "datetime", helpText, setter),
		cfg: c,
	}
}

// panics for invalid values
func (c *Config) NewJSON(name, helpText string, setter ...func(*Option)) *JSONGetter {
	return &JSONGetter{
		opt: c.mkOption(name, "json", helpText, setter),
		cfg: c,
	}
}

// panics for invalid values
func Required(o *Option) { o.Required = true }

// panics for invalid values
func Default(val interface{}) func(*Option) {
	return func(o *Option) { o.Default = val }
}

// panics for invalid values
func Shortflag(s rune) func(*Option) {
	return func(o *Option) { o.Shortflag = strings.ToUpper(string(s)) }
}

// panics for invalid values
func (c *Config) mkOption(name, type_, helpText string, setter []func(*Option)) *Option {
	o := &Option{Name: strings.ToUpper(name), Type: type_, Help: helpText}

	for _, s := range setter {
		s(o)
	}

	if err := o.Validate(); err != nil {
		panic(err)
	}

	c.addOption(o)
	return o
}

type Option struct {
	// Name must consist of words that are joined by the underscore character _
	// Each word must consist of uppercase letters [A-Z] and may have numbers
	// A word must consist of two ascii characters or more.
	// A name must at least have one word
	Name string `json:"name"`

	// Required indicates, if the Option is required
	Required bool `json:"required"`

	// Type must be one of "bool","int32","float32","string","datetime","json"
	Type string `json:"type"`

	// The Help string is part of the documentation
	Help string `json:"help"`

	// The Default value for the Config. The value might be nil for optional Options.
	// Otherwise, it must have the same type as the Type property indicates
	Default interface{} `json:"default,omitempty"`

	// A Shortflag for the Option. Shortflags may only be used for commandline flags
	// They must be a single lowercase ascii character
	Shortflag string `json:"shortflag,omitempty"`
}

// ValidateDefault checks if the default value is valid.
// If it does, nil is returned, otherwise
// ErrInvalidDefault is returned or a json unmarshalling error if the type is json
func (c Option) ValidateDefault() error {
	err := c.ValidateValue(c.Default)
	if err != nil {
		return ErrInvalidDefault
	}
	return nil
}

// ValidateValue checks if the given value is valid.
// If it does, nil is returned, otherwise
// ErrInvalidValue is returned or a json unmarshalling error if the type is json
func (c Option) ValidateValue(val interface{}) error {
	// value may only be nil for optional Options
	if val == nil && c.Required {
		return ErrInvalidValue
	}

	if val == nil {
		return nil
	}
	switch ty := val.(type) {
	case bool:
		if c.Type != "bool" {
			return ErrInvalidValue
		}
	case int32:
		if c.Type != "int32" {
			return ErrInvalidValue
		}
	case float32:
		if c.Type != "float32" {
			return ErrInvalidValue
		}
	case string:
		if c.Type != "string" && c.Type != "json" {
			return ErrInvalidValue
		}
		if c.Type == "json" {
			var v interface{}
			if err := json.Unmarshal([]byte(ty), &v); err != nil {
				return err
			}
		}
	case time.Time:
		if c.Type != "datetime" {
			return ErrInvalidValue
		}
	default:
		return ErrInvalidValue
	}
	return nil
}

// Validate checks if the Option is valid.
// If it does, nil is returned, otherwise
// the error is returned
func (c Option) Validate() error {
	if err := ValidateName(c.Name); err != nil {
		return err
	}
	if err := ValidateType(c.Type); err != nil {
		return err
	}
	if err := c.ValidateDefault(); err != nil {
		return err
	}
	if c.Help == "" {
		return ErrMissingHelp
	}
	return nil
}
