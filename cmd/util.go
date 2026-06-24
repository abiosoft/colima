package cmd

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/app"
	"github.com/abiosoft/colima/cli"
	"github.com/sirupsen/logrus"
)

func newApp() app.App {
	colimaApp, err := app.New()
	if err != nil {
		logrus.Fatal("Error: ", err)
	}
	return colimaApp
}

// waitForUserEdit launches a temporary file with content using editor,
// and waits for the user to close the editor.
// It returns the filename (if saved), empty file name (if aborted), and an error (if any).
func waitForUserEdit(editor string, content []byte) (string, error) {
	tmp, err := os.CreateTemp("", "colima-*.yaml")
	if err != nil {
		return "", fmt.Errorf("error creating temporary file: %w", err)
	}
	if _, err := tmp.Write(content); err != nil {
		return "", fmt.Errorf("error writing temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("error closing temporary file: %w", err)
	}

	if err := launchEditor(editor, tmp.Name()); err != nil {
		return "", err
	}

	// aborted
	if f, err := os.ReadFile(tmp.Name()); err == nil && len(bytes.TrimSpace(f)) == 0 {
		return "", nil
	}

	return tmp.Name(), nil
}

var editors = []string{
	"vim",
	"code --wait --new-window",
	"nano",
}

func launchEditor(editor string, file string) error {
	if editor != "" {
		log.Println("editing in", editor)
	}
	// if not specified, prefer vscode if this a vscode terminal
	if editor == "" {
		if os.Getenv("TERM_PROGRAM") == "vscode" {
			log.Println("vscode detected, editing in vscode")
			editor = "code --wait"
		}
	}

	// if not found, check the EDITOR env var
	if editor == "" {
		if e := os.Getenv("EDITOR"); e != "" {
			log.Println("editing in", e, "from", "$EDITOR environment variable")
			editor = e
		}
	}

	// if not found, check the preferred editors
	if editor == "" {
		for _, e := range editors {
			s := strings.Fields(e)
			if _, err := exec.LookPath(s[0]); err == nil {
				editor = e
				log.Println("editing in", e)
				break
			}
		}
	}

	// if still not found, abort
	if editor == "" {
		return fmt.Errorf("no editor found in $PATH, kindly set $EDITOR environment variable and try again")
	}

	// some editors need the wait flag, let us add it if the user has not.
	switch editor {
	case "code", "code-insiders", "code-oss", "codium", "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code":
		editor = strconv.Quote(editor) + " --wait --new-window"
	case "mate", "/Applications/TextMate 2.app/Contents/MacOS/mate", "/Applications/TextMate 2.app/Contents/MacOS/TextMate":
		editor = strconv.Quote(editor) + " --wait"
	}

	return cli.CommandInteractive("sh", "-c", editor+" "+file).Run()
}
