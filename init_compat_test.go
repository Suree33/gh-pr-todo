package main

import (
	"io"

	"charm.land/huh/v2"
	"github.com/Suree33/gh-pr-todo/internal/initcmd"
	"github.com/fatih/color"
	"github.com/spf13/pflag"
)

type initTarget = initcmd.Target

const (
	initTargetPrompt = initcmd.TargetPrompt
	initTargetRepo   = initcmd.TargetRepo
	initTargetGlobal = initcmd.TargetGlobal
)

func initTargetFromFlags(repoFlag, globalFlag bool) (initTarget, error) {
	return initcmd.TargetFromFlags(repoFlag, globalFlag)
}

func runInit(in io.Reader, out io.Writer, cwd, userConfigDir string, force bool, target initTarget) error {
	return initcmd.Run(in, out, cwd, userConfigDir, force, target)
}

func initPathOptions(repoPath string, repoErr error, globalPath string, globalErr error) []huh.Option[string] {
	return initcmd.PathOptions(repoPath, repoErr, globalPath, globalErr)
}

func chooseInitPathText(in io.Reader, out io.Writer, repoPath string, repoErr error, globalPath string, globalErr error) (string, error) {
	return initcmd.ChoosePathText(in, out, repoPath, repoErr, globalPath, globalErr)
}

func shouldUseInteractivePrompt(in io.Reader, out io.Writer) bool {
	return initcmd.ShouldUseInteractivePrompt(in, out)
}

func printInitUsage(fs *pflag.FlagSet) {
	initcmd.PrintUsage(color.Output, fs)
}
