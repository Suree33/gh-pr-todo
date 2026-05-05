package initcmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"charm.land/huh/v2"
	"github.com/Suree33/gh-pr-todo/internal/config"
	"github.com/Suree33/gh-pr-todo/internal/output"
	"github.com/spf13/pflag"
)

type Target int

const (
	TargetPrompt Target = iota
	TargetRepo
	TargetGlobal
)

const (
	ProjectUnavailableLabel = "Project (unavailable: not inside a Git repository)"
	GlobalUnavailableLabel  = "Global (unavailable: user config directory not available)"
)

type Flags struct {
	Force  bool
	Repo   bool
	Global bool
	Help   bool
}

type Command struct {
	In            io.Reader
	Out           io.Writer
	ErrOut        io.Writer
	UsageOut      io.Writer
	Getwd         func() (string, error)
	UserConfigDir func() (string, error)

	chooser chooser
}

type chooser struct {
	useInteractive func(io.Reader, io.Writer) bool
	interactive    func(io.Reader, io.Writer, string, error, string, error) (string, error)
	text           func(io.Reader, io.Writer, string, error, string, error) (string, error)
}

func newChooser() chooser {
	return chooser{
		useInteractive: ShouldUseInteractivePrompt,
		interactive:    ChoosePathInteractive,
		text:           ChoosePathText,
	}
}

func (c chooser) choose(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
	if c.useInteractive(in, out) {
		return c.interactive(in, out, repoPath, repoErr, globalPath, globalErr)
	}
	return c.text(in, out, repoPath, repoErr, globalPath, globalErr)
}

func NewFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("gh pr-todo init", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Bool("force", false, "Overwrite existing config file")
	fs.Bool("repo", false, "Create repo config at <repo>/.gh-pr-todo.yml without prompting")
	fs.Bool("global", false, "Create global config at user config dir/gh-pr-todo/config.yml without prompting")
	fs.BoolP("help", "h", false, "Display help information")
	return fs
}

func ParseFlags(fs *pflag.FlagSet, args []string) (Flags, error) {
	if err := fs.Parse(args); err != nil {
		return Flags{}, err
	}
	force, err := fs.GetBool("force")
	if err != nil {
		return Flags{}, err
	}
	repo, err := fs.GetBool("repo")
	if err != nil {
		return Flags{}, err
	}
	global, err := fs.GetBool("global")
	if err != nil {
		return Flags{}, err
	}
	help, err := fs.GetBool("help")
	if err != nil {
		return Flags{}, err
	}
	return Flags{Force: force, Repo: repo, Global: global, Help: help}, nil
}

func (c Command) Execute(args []string) int {
	fs := NewFlagSet()
	flags, err := ParseFlags(fs, args)
	if err != nil {
		fmt.Fprintln(c.errWriter(), err)
		return 1
	}

	if flags.Help {
		PrintUsage(c.usageWriter(), fs)
		return 0
	}

	if fs.NArg() > 0 {
		fmt.Fprintln(c.errWriter(), "gh pr-todo init: unexpected argument")
		PrintUsage(c.usageWriter(), fs)
		return 1
	}

	target, err := TargetFromFlags(flags.Repo, flags.Global)
	if err != nil {
		fmt.Fprintln(c.errWriter(), err)
		return 1
	}

	cwd := ""
	if target != TargetGlobal {
		cwd, err = c.getcwd()
		if err != nil {
			fmt.Fprintln(c.errWriter(), "Error getting current directory:", err)
			return 1
		}
	}

	userConfigDir, _ := c.userConfigDir()
	if err := c.run(c.In, c.outWriter(), cwd, userConfigDir, flags.Force, target); err != nil {
		fmt.Fprintln(c.errWriter(), err)
		return 1
	}
	return 0
}

func (c Command) run(in io.Reader, out io.Writer, cwd, userConfigDir string, force bool, target Target) error {
	r := runner{chooser: c.pathChooser()}
	return r.run(in, out, cwd, userConfigDir, force, target)
}

func (c Command) pathChooser() chooser {
	if c.chooser.useInteractive == nil || c.chooser.interactive == nil || c.chooser.text == nil {
		return newChooser()
	}
	return c.chooser
}

func (c Command) getcwd() (string, error) {
	if c.Getwd != nil {
		return c.Getwd()
	}
	return os.Getwd()
}

func (c Command) userConfigDir() (string, error) {
	if c.UserConfigDir != nil {
		return c.UserConfigDir()
	}
	return os.UserConfigDir()
}

func (c Command) outWriter() io.Writer {
	if c.Out != nil {
		return c.Out
	}
	return os.Stdout
}

func (c Command) errWriter() io.Writer {
	if c.ErrOut != nil {
		return c.ErrOut
	}
	return os.Stderr
}

func (c Command) usageWriter() io.Writer {
	if c.UsageOut != nil {
		return c.UsageOut
	}
	return c.outWriter()
}

type runner struct {
	chooser chooser
}

func Run(in io.Reader, out io.Writer, cwd, userConfigDir string, force bool, target Target) error {
	return runner{chooser: newChooser()}.run(in, out, cwd, userConfigDir, force, target)
}

func (r runner) run(in io.Reader, out io.Writer, cwd, userConfigDir string, force bool, target Target) error {
	globalPath, globalErr := config.GlobalPath(userConfigDir)
	repoPath := ""
	repoErr := fmt.Errorf("repo config requires a Git repository")
	if target != TargetGlobal {
		repoPath, repoErr = config.RepoRootPath(cwd)
	}

	path, err := r.resolvePath(in, out, repoPath, repoErr, globalPath, globalErr, target)
	if err != nil {
		return err
	}

	if path == repoPath {
		if err := EnsureNoRepoNarrowConfig(cwd, repoPath); err != nil {
			return err
		}
	}

	if err := config.WriteDefault(path, force); err != nil {
		return err
	}

	fmt.Fprintf(out, "Created %s\n", path)
	return nil
}

func (r runner) resolvePath(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error, target Target) (string, error) {
	switch target {
	case TargetRepo:
		if repoErr != nil {
			return "", fmt.Errorf("repo config requires a Git repository")
		}
		return repoPath, nil
	case TargetGlobal:
		if globalErr != nil || globalPath == "" {
			return "", fmt.Errorf("user config directory not available")
		}
		return globalPath, nil
	}
	return r.chooser.choose(in, out, repoPath, repoErr, globalPath, globalErr)
}

func TargetFromFlags(repoFlag, globalFlag bool) (Target, error) {
	switch {
	case repoFlag && globalFlag:
		return TargetPrompt, fmt.Errorf("cannot use --repo and --global together")
	case repoFlag:
		return TargetRepo, nil
	case globalFlag:
		return TargetGlobal, nil
	default:
		return TargetPrompt, nil
	}
}

func EnsureNoRepoNarrowConfig(cwd, repoPath string) error {
	narrowPath, err := config.RepoNarrowPath(cwd)
	if err != nil {
		return nil
	}
	if _, err := os.Stat(narrowPath); err == nil {
		return fmt.Errorf("%s already exists and takes precedence over %s; remove it or move it before running init", narrowPath, repoPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking %s: %w", narrowPath, err)
	}
	return nil
}

func ResolvePath(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error, target Target) (string, error) {
	return runner{chooser: newChooser()}.resolvePath(in, out, repoPath, repoErr, globalPath, globalErr, target)
}

func ChoosePath(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
	return newChooser().choose(in, out, repoPath, repoErr, globalPath, globalErr)
}

func ChoosePathInteractive(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
	options := PathOptions(repoPath, repoErr, globalPath, globalErr)
	if len(options) == 0 {
		return "", fmt.Errorf("no config file location available")
	}

	path := options[0].Value
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose config file location").
				Options(options...).
				Value(&path),
		),
	).WithInput(in).WithOutput(out)

	if err := form.Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", fmt.Errorf("init aborted")
		}
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return path, nil
}

func PathOptions(repoPath string, repoErr error, globalPath string, globalErr error) []huh.Option[string] {
	options := make([]huh.Option[string], 0, 2)
	if repoErr == nil {
		options = append(options, huh.NewOption(ProjectLabel(), repoPath))
	}
	if globalErr == nil && globalPath != "" {
		options = append(options, huh.NewOption(GlobalLabel(globalPath), globalPath))
	}
	return options
}

func ProjectLabel() string {
	return "Project (.gh-pr-todo.yml)"
}

func GlobalLabel(path string) string {
	return fmt.Sprintf("Global (%s)", path)
}

func ChoosePathText(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
	if repoErr != nil && (globalErr != nil || globalPath == "") {
		return "", fmt.Errorf("no config file location available")
	}

	fmt.Fprintln(out, "Choose config file location:")
	if repoErr == nil {
		fmt.Fprintf(out, "  1) %s\n", ProjectLabel())
	} else {
		fmt.Fprintf(out, "  1) %s\n", ProjectUnavailableLabel)
	}
	if globalErr == nil && globalPath != "" {
		fmt.Fprintf(out, "  2) %s\n", GlobalLabel(globalPath))
	} else {
		fmt.Fprintf(out, "  2) %s\n", GlobalUnavailableLabel)
	}
	fmt.Fprint(out, "Enter selection: ")

	reader := bufio.NewReader(in)
	line, err := reader.ReadString('\n')
	if err != nil && (err != io.EOF || line == "") {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	line = strings.TrimSpace(line)

	switch line {
	case "1":
		if repoErr != nil {
			return "", fmt.Errorf("repo config requires a Git repository")
		}
		return repoPath, nil
	case "2":
		if globalErr != nil || globalPath == "" {
			return "", fmt.Errorf("user config directory not available")
		}
		return globalPath, nil
	case "":
		return "", fmt.Errorf("no input received")
	default:
		return "", fmt.Errorf("invalid selection %q: enter 1 or 2", line)
	}
}

func ShouldUseInteractivePrompt(in io.Reader, out io.Writer) bool {
	return IsTerminalFile(in) && IsTerminalFile(out)
}

func IsTerminalFile(v any) bool {
	file, ok := v.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func PrintUsage(out io.Writer, fs *pflag.FlagSet) {
	fmt.Fprintf(out, "%s\n\n", "Create a default config file.")
	fmt.Fprintf(out, "%s\n", output.Bold("USAGE"))
	fmt.Fprintf(out, "  %s\n\n", "gh pr-todo init [--repo | --global] [--force]")
	fmt.Fprintf(out, "%s\n", output.Bold("FLAGS"))
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Shorthand != "" {
			fmt.Fprintf(out, "  -%s, --%s  %s\n", f.Shorthand, f.Name, f.Usage)
		} else {
			fmt.Fprintf(out, "      --%s  %s\n", f.Name, f.Usage)
		}
	})
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s\n", output.Bold("DESCRIPTION"))
	fmt.Fprintf(out, "  %s\n", "Creates a default configuration file. Without --repo or --global, init prompts for a location.")
	fmt.Fprintf(out, "  %s\n", "Locations:")
	fmt.Fprintf(out, "  %s\n", "  - --repo: Project (.gh-pr-todo.yml), available inside a Git repository")
	fmt.Fprintf(out, "  %s\n", "  - --global: user config dir/gh-pr-todo/config.yml")
	fmt.Fprintf(out, "  %s\n", "")
	fmt.Fprintf(out, "  %s\n", "Use --force to overwrite an existing project or global config file.")
	fmt.Fprintf(out, "  %s\n", "If you choose repo scope and .github/gh-pr-todo.yml exists, move or remove it first because it takes precedence.")
	fmt.Fprintln(out)
}
