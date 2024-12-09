package config

import (
	"flag"
	"log"
	"strings"
)

type StringSlice struct {
	Values      []string
	wipeDefault bool // Track whether the default value is currently in use
}

func (s *StringSlice) String() string {
	return strings.Join(s.Values, ", ")
}

func (s *StringSlice) Set(value string) error {
	if s.wipeDefault {
		s.Values = nil
		s.wipeDefault = false
	}

	values := strings.Split(value, ",")
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			s.Values = append(s.Values, v)
		}
	}
	return nil
}

type Config struct {
	DatabasePath StringSlice
	StartingDirs StringSlice
	IgnoreStr    StringSlice
	IncludeStr   StringSlice
	IgnoreExt    StringSlice
	IncludeExt   StringSlice
	SaveSC       bool
	AbsPath      bool
}

func (c *Config) ParseArgs() {
	c.DatabasePath = StringSlice{Values: []string{"./videos.db"}, wipeDefault: true}
	c.StartingDirs = StringSlice{Values: []string{"."}, wipeDefault: true}
	c.IgnoreStr = StringSlice{}
	c.IncludeStr = StringSlice{}
	c.IgnoreExt = StringSlice{}
	c.IncludeExt = StringSlice{Values: []string{"mp4", "m4a"}, wipeDefault: false}
	c.SaveSC = false
	c.AbsPath = true

	flag.Var(&c.DatabasePath, "dp", "Specify database path where the database will live. Default value is \"./videos.db\".")
	flag.Var(&c.StartingDirs, "sd", "Specify directory path(s) where the search will begin from (use multiple times for multiple Directories). Default value is \".\".")
	flag.Var(&c.IgnoreStr, "igs", "Specify string(s) to ignore (use multiple times for multiple strings). Default value is \"\".")
	flag.Var(&c.IncludeStr, "is", "Specify string(s) to include (use multiple times for multiple strings). Default value is \"\".")
	flag.Var(&c.IgnoreExt, "ige", "Specify extension(s) to ignore (use multiple times for multiple ext). Default value is \"\".")
	flag.Var(&c.IncludeExt, "ie", "Specify extension(s) to include (use multiple times for multiple ext). Default value is \"mp4,m4a\".")
	c.SaveSC = *flag.Bool("sc", true, "Flag to save screenshots to folder T/F. Default value is \"False\".")
	c.AbsPath = *flag.Bool("ap", true, "T/F. Default value is \"True\".")

	flag.Parse()

	log.Println("DatabasePath:", c.DatabasePath.Values)
	log.Println("StartingDirs:", c.StartingDirs.Values)
	log.Println("Ignore File Strings:", c.IgnoreStr.Values)
	log.Println("Include File Strings:", c.IncludeStr.Values)
	log.Println("Ignore File Extensions:", c.IgnoreExt.Values)
	log.Println("Include File Extensions:", c.IncludeExt.Values)
	log.Println("Screenshot flag:", c.SaveSC)
	log.Println("AbsPath flag:", c.AbsPath)
}
