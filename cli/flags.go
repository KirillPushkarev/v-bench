package cli

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"regexp"
	"strings"
)

var (
	levels = regexp.MustCompile("^(debug|info|warn|error|fatal)$")
	colors = regexp.MustCompile("^(no)?colou?rs?$")
	json   = regexp.MustCompile("^json$")
)

// SliceValue stores multi-value command line arguments.
type SliceValue []string

// String makes SliceValue implement flag.Value interface.
func (s *SliceValue) String() string {
	return fmt.Sprintf("%s", *s)
}

// Set makes SliceValue implement flag.Value interface.
func (s *SliceValue) Set(value string) error {
	for _, v := range strings.Split(value, ",") {
		if len(v) > 0 {
			*s = append(*s, v)
		}
	}
	return nil
}

func ApplyLogFlags(logFlags SliceValue) {
	log.SetLevel(log.InfoLevel)

	for _, f := range logFlags {
		if levels.MatchString(f) {
			lvl, err := log.ParseLevel(f)
			if err != nil {
				// Should never happen since we select correct levels
				// Unless logrus commits a breaking change on level names
				panic(fmt.Errorf("invalid log level: %s", err.Error()))
			}
			log.SetLevel(lvl)
		} else if colors.MatchString(f) {
			if f[:2] == "no" {
				log.SetFormatter(&log.TextFormatter{DisableColors: true})
			} else {
				log.SetFormatter(&log.TextFormatter{ForceColors: true})
			}
		} else if json.MatchString(f) {
			log.SetFormatter(&log.JSONFormatter{})
		}
	}
}
