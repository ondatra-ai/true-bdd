package main

import "errors"

var (
	errNoCommand      = errors.New("no command given")
	errUnknownCommand = errors.New("unknown command")
	errMissingFlag    = errors.New("a required flag is missing")
	errNoTicketBound  = errors.New("no Ticket is bound to this Task — nothing to close")
)
