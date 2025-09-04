package main

import log "github.com/sirupsen/logrus"

func noerror(err error, args ...any) {
	if err != nil {
		log.WithError(err).Fatal(args...)
	}
}

func logerrorf(err error, msg string, args ...any) {
	if err != nil {
		log.WithError(err).Errorf(msg, args...)
	}
}

func logwarningf(err error, msg string, args ...any) {
	if err != nil {
		log.WithError(err).Warningf(msg, args...)
	}
}
