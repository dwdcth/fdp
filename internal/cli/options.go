package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type StringList []string

func (s *StringList) String() string {
	return strings.Join(*s, ",")
}

func (s *StringList) Set(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("mirror cannot be empty")
	}
	*s = append(*s, trimmed)
	return nil
}

type Options struct {
	ImageRef  string
	Mirrors   []string
	Workers   int
	AriaSplit int
	AriaConn  int
	Output    string
	Format    string
	Platform  string
	Aria2c    string
	CacheDir  string
	StateDB   string
}

func Parse(args []string) (Options, error) {
	opts := Options{}
	imageRef, flagArgs, err := splitImageRefArgs(args)
	if err != nil {
		return opts, err
	}

	fs := flag.NewFlagSet("dockerpull", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var mirrors StringList
	var useDocker bool
	var useOCI bool

	fs.Var(&mirrors, "m", "registry mirror host, repeatable")
	fs.IntVar(&opts.Workers, "t", 4, "layer worker count")
	fs.IntVar(&opts.AriaConn, "x", 16, "aria2c max connections per server")
	fs.IntVar(&opts.AriaSplit, "s", 16, "aria2c split count")
	fs.StringVar(&opts.Output, "o", "", "output archive path")
	fs.StringVar(&opts.Platform, "platform", "linux/amd64", "target platform os/arch[/variant]")
	fs.StringVar(&opts.Aria2c, "aria2c", "aria2c", "aria2c executable path")
	fs.StringVar(&opts.CacheDir, "cache-dir", filepath.Join(".", "cache"), "cache directory")
	fs.StringVar(&opts.StateDB, "state-db", "", "sqlite state db path")
	fs.BoolVar(&useDocker, "docker", false, "export docker archive")
	fs.BoolVar(&useOCI, "oci", false, "export oci archive")

	if err := fs.Parse(flagArgs); err != nil {
		return opts, err
	}
	if fs.NArg() != 0 {
		return opts, fmt.Errorf("usage: dockerpull IMAGE[:TAG] [flags]")
	}
	if useDocker && useOCI {
		return opts, errors.New("--docker and --oci are mutually exclusive")
	}
	if opts.Output == "" {
		return opts, errors.New("-o is required")
	}
	if opts.Workers <= 0 {
		return opts, errors.New("-t must be > 0")
	}
	if opts.AriaConn <= 0 {
		return opts, errors.New("-x must be > 0")
	}
	if opts.AriaSplit <= 0 {
		return opts, errors.New("-s must be > 0")
	}

	opts.ImageRef = imageRef
	opts.Format = "docker"
	if useOCI {
		opts.Format = "oci"
	} else if useDocker {
		opts.Format = "docker"
	}

	mirrors = append(mirrors, parseEnvMirrors(os.Getenv("DOCKER_MIRRORS"))...)
	opts.Mirrors = normalizeMirrors(mirrors)
	if opts.StateDB == "" {
		opts.StateDB = filepath.Join(opts.CacheDir, "state", "images.db")
	}
	if opts.Platform == "" {
		opts.Platform = defaultPlatform()
	}
	return opts, nil
}

func splitImageRefArgs(args []string) (string, []string, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("usage: dockerpull IMAGE[:TAG] [flags]")
	}
	for i, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		flagArgs := make([]string, 0, len(args)-1)
		flagArgs = append(flagArgs, args[:i]...)
		flagArgs = append(flagArgs, args[i+1:]...)
		return arg, flagArgs, nil
	}
	return "", nil, fmt.Errorf("usage: dockerpull IMAGE[:TAG] [flags]")
}

func defaultPlatform() string {
	if runtime.GOOS == "linux" && runtime.GOARCH == "amd64" {
		return "linux/amd64"
	}
	return "linux/amd64"
}

func parseEnvMirrors(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeMirrors(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, mirror := range in {
		trimmed := strings.TrimRight(strings.TrimSpace(mirror), "/")
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
