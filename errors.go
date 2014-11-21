package config

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidName      = errors.New("invalid name")
	ErrInvalidVersion   = errors.New("invalid version")
	ErrInvalidShortflag = errors.New("invalid shortflag")

	ErrInvalidType    = errors.New("invalid type")
	ErrInvalidDefault = errors.New("invalid default")
	ErrInvalidValue   = errors.New("invalid value")
	ErrMissingHelp    = errors.New("missing help text")
)

type ErrInvalidOptionName string

func (e ErrInvalidOptionName) Error() string {
	return fmt.Sprintf("invalid option name %s", string(e))
}

type ErrInvalidAppName string

func (e ErrInvalidAppName) Error() string {
	return fmt.Sprintf("invalid app name %s", string(e))
}

type ErrUnknownOption string

func (e ErrUnknownOption) Error() string {
	return fmt.Sprintf("unknown option %s", string(e))
}

type ErrDoubleOption string

func (e ErrDoubleOption) Error() string {
	return fmt.Sprintf("option %s is set twice", string(e))
}

type ErrDoubleShortflag string

func (e ErrDoubleShortflag) Error() string {
	return fmt.Sprintf("shortflag %s is set twice", string(e))
}
