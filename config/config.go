package config

import (
	"flag"
	"log"
	"strings"
)

type StringSlice []string

func (s *StringSlice) String() string {
	return strings.Join(*s, ", ")
}

func (s *StringSlice) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type Config struct {
	StartingDirs StringSlice
	IgnoreStr    StringSlice
	IncludeStr   StringSlice
	IgnoreExt    StringSlice
	IncludeExt   StringSlice
	SaveSC       bool
}

func (c *Config) ParseArgs() {
	flag.Var(&c.StartingDirs, "d", "Specify directory path(s) where the search will begin from (use multiple times for multiple Directories)")
	flag.Var(&c.IgnoreStr, "igs", "Specify string(s) to ignore (use multiple times for multiple strings)")
	flag.Var(&c.IncludeStr, "is", "Specify string(s) to include (use multiple times for multiple strings)")
	flag.Var(&c.IgnoreExt, "ige", "Specify extension(s) to ignore (use multiple times for multiple ext)")
	flag.Var(&c.IncludeExt, "ie", "Specify extension(s) to include (use multiple times for multiple ext)")
	c.SaveSC = *flag.Bool("sc", true, "Flag to save screenshots to folder T/F")
	flag.Parse()

	log.Println("StartingDirs:", c.StartingDirs)
	log.Println("Ignore File Strings:", c.IgnoreStr)
	log.Println("Include File Strings:", c.IgnoreStr)
	log.Println("Ignore File Extensions:", c.IgnoreExt)
	log.Println("Include File Extensions:", c.IncludeExt)
	log.Println("Screenshot flag:", c.SaveSC)
}
