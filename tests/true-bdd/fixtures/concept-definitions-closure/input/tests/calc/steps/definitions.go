package steps

import "errors"

var errNotWritten = errors.New("not written")

func halving(*State, []string) error { return errNotWritten }

//truebdd:concept when "doubling"
func doubling(*State, []string) error { return errNotWritten }

//truebdd:concept when "doubling"
func doublingSix(*State, []string) error { return errNotWritten }

//truebdd:concept then "result"
func result(*State, []string) error { return errNotWritten }

//truebdd:concept then "halving"
func even(*State, []string) error { return errNotWritten }
