package args

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
	Directories StringSlice
	IgnoreStr   StringSlice
	IncludeStr  StringSlice
	IgnoreExt   StringSlice
	IncludeExt  StringSlice
}

func (c *Config) ParseArgs() {
	flag.Var(&c.Directories, "d", "Specify directory path(s) (use multiple times for multiple Directories)")
	flag.Var(&c.IgnoreStr, "igs", "Specify string(s) to ignore (use multiple times for multiple strings)")
	flag.Var(&c.IncludeStr, "is", "Specify string(s) to include (use multiple times for multiple strings)")
	flag.Var(&c.IgnoreExt, "ige", "Specify extension(s) to ignore (use multiple times for multiple ext)")
	flag.Var(&c.IncludeExt, "ie", "Specify extension(s) to include (use multiple times for multiple ext)")
	flag.Parse()

	log.Println("Directories:", c.Directories)
	log.Println("Ignore File Strings:", c.IgnoreStr)
	log.Println("Include File Strings:", c.IgnoreStr)
	log.Println("Ignore File Extensions:", c.IgnoreExt)
	log.Println("Include File Extensions:", c.IncludeExt)
}
