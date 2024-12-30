package config

import (
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
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
	DatabasePath        StringSlice
	StartingDirs        StringSlice
	IgnoreStr           StringSlice
	IncludeStr          StringSlice
	IgnoreExt           StringSlice
	IncludeExt          StringSlice
	SaveSC              bool
	AbsPath             bool
	FollowSymbolicLinks bool
}

func (c *Config) ParseArgs() {
	c.DatabasePath = StringSlice{Values: []string{"./videos.db"}, wipeDefault: true}
	c.StartingDirs = StringSlice{Values: []string{"."}, wipeDefault: true}
	c.IgnoreStr = StringSlice{}
	c.IncludeStr = StringSlice{}
	c.IgnoreExt = StringSlice{}
	c.IncludeExt = StringSlice{Values: []string{"mp4", "m4a", "webm"}, wipeDefault: false}
	c.SaveSC = false
	c.AbsPath = true
	c.FollowSymbolicLinks = false

	flag.Var(&c.DatabasePath, "dp", "Specify database path where the database will live. Default value is \"./videos.db\".")
	flag.Var(&c.StartingDirs, "sd", "Specify directory path(s) where the search will begin from (use multiple times for multiple Directories). Default value is \".\".")
	flag.Var(&c.IgnoreStr, "igs", "Specify string(s) to ignore (use multiple times for multiple strings). Default value is \"\".")
	flag.Var(&c.IncludeStr, "is", "Specify string(s) to include (use multiple times for multiple strings). Default value is \"\".")
	flag.Var(&c.IgnoreExt, "ige", "Specify extension(s) to ignore (use multiple times for multiple ext). Default value is \"\".")
	flag.Var(&c.IncludeExt, "ie", "Specify extension(s) to include (use multiple times for multiple ext). Default value is \"mp4,m4a\".")
	c.SaveSC = *flag.Bool("sc", true, "Flag to save screenshots to folder T/F. Default value is \"False\".")
	c.FollowSymbolicLinks = *flag.Bool("fsl", true, "T/F. Default value is \"False\".")

	flag.Parse()

	// get full path for startingdirs, ie: ./ or . to /path/path
	validateStartingDirs(c)

	log.Println("DatabasePath:", c.DatabasePath.Values)
	log.Println("StartingDirs:", c.StartingDirs.Values)
	log.Println("Ignore File Strings:", c.IgnoreStr.Values)
	log.Println("Include File Strings:", c.IncludeStr.Values)
	log.Println("Ignore File Extensions:", c.IgnoreExt.Values)
	log.Println("Include File Extensions:", c.IncludeExt.Values)
	log.Println("Screenshot flag:", c.SaveSC)
	log.Println("FollowSymbolicLinks:", c.FollowSymbolicLinks)
}

// no return, exit if error
// fix starting dir values .. . (relative paths fixed to absolute)
// clean paths
func validateStartingDirs(c *Config) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error getting wd, error: %v", err)
	}
	log.Println(wd)

	for i, dir := range c.StartingDirs.Values {
		f, err := os.Open(dir)
		if err != nil {
			log.Fatalf("error opening dir, dir: %q", dir)
		}
		log.Println(c.StartingDirs.Values[i])

		abs, err := filepath.Abs(dir)
		if err != nil {
			log.Fatalf("error getting the absolute path for dir: %q, err: %v", dir, err)
		}
		c.StartingDirs.Values[i] = abs

		// check if dir exists
		fsInfo, err := f.Stat()
		if errors.Is(err, os.ErrNotExist) {
			log.Fatalf("error calling stat on dir: %q dir does not exist, err: %v", dir, err)
		} else if err != nil {
			log.Fatalf("error calling stat on dir, not a os.ErrNotExist error. dir %q, err: %v", dir, err)
		}
		if !fsInfo.IsDir() {
			log.Fatalf("error dir: %q is not a valid directory", dir)
		}
	}
}

/*
func createStartingDirs(c *Config) {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error getting working directory, error: %v", err)
	}

	for _, d := range c.StartingDirs.Values {
		if strings.Contains(d, ".") {
            d = wd :
		}
	}
}
*/
