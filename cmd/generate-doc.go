package cmd

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newGenDocCommand() *cobra.Command {
	genmanCommand := &cobra.Command{
		Use:    "generate-doc DIR",
		Short:  "Generate cli-reference pages",
		Args:   cobra.ExactArgs(1),
		RunE:   gendocAction,
		Hidden: true,
	}
	genmanCommand.Flags().String("type", "docsy", "Output type (docsy)")
	return genmanCommand
}

func gendocAction(cmd *cobra.Command, args []string) error {
	outputType, err := cmd.Flags().GetString("type")
	if err != nil {
		return err
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := args[0]

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	switch outputType {
	case "docsy":
		if err := genDocsy(cmd, dir); err != nil {
			return err
		}
	}
	return replaceAll(dir, homeDir, "~")
}

func genDocsy(cmd *cobra.Command, dir string) error {
	return doc.GenMarkdownTreeCustom(cmd.Root(), dir, func(s string) string {
		name := filepath.Base(s)
		name = strings.ReplaceAll(name, "colima", "")
		name = strings.ReplaceAll(name, "_", " ")
		name = strings.TrimSuffix(name, filepath.Ext(name))
		return fmt.Sprintf(`---
title: %s
weight: 3
---
`, name)
	}, func(s string) string {
		return strings.TrimSuffix(s, filepath.Ext(s))
	})
}

func replaceAll(dir, text, replacement string) error {
	logrus.Infof("Replacing %q with %q", text, replacement)
	return filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		if info.IsDir() {
			return filepath.SkipDir
		}
		in, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}
		out := bytes.ReplaceAll(in, []byte(text), []byte(replacement))
		err = os.WriteFile(path, out, 0o644)
		if err != nil {
			return fmt.Errorf("failed to write file %s: %w", path, err)
		}
		return nil
	})
}

func init() {
	root.Cmd().AddCommand(newGenDocCommand())
}
