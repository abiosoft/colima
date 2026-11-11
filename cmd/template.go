package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/util/yamlutil"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// templateCmd represents the template command
var templateCmd = &cobra.Command{
	Use:     "template",
	Aliases: []string{"tmpl", "tpl", "t"},
	Short:   "edit the template for default configurations",
	Long: `Edit the template for default configurations of new instances.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if templateCmdArgs.Print {
			fmt.Println(templateFile())
			return nil
		}
		// there are unwarranted []byte to string overheads.
		// not a big deal in this case

		abort, err := embedded.ReadString("defaults/abort.yaml")
		if err != nil {
			return fmt.Errorf("error reading embedded file: %w", err)
		}
		info, err := embedded.ReadString("defaults/template.yaml")
		if err != nil {
			return fmt.Errorf("error reading embedded file: %w", err)
		}
		template, err := templateFileOrDefault()
		if err != nil {
			return fmt.Errorf("error reading template file: %w", err)
		}

		tmpFile, err := waitForUserEdit(templateCmdArgs.Editor, []byte(abort+"\n"+info+"\n"+template))
		if err != nil {
			return fmt.Errorf("error editing template file: %w", err)
		}
		if tmpFile == "" {
			return fmt.Errorf("empty file, template edit aborted")
		}
		defer func() {
			_ = os.Remove(tmpFile)
		}()

		// load and resave template to ensure the format is correct
		cf, err := configmanager.LoadFrom(tmpFile)
		if err != nil {
			return fmt.Errorf("error in template: %w", err)
		}
		if err := configmanager.SaveToFile(cf, templateFile()); err != nil {
			return fmt.Errorf("error saving template: %w", err)
		}

		log.Println("configurations template saved")

		return nil
	},
}

func templateFile() string { return filepath.Join(config.TemplatesDir(), "default.yaml") }

func templateFileOrDefault() (string, error) {
	tFile := templateFile()
	if _, err := os.Stat(tFile); err == nil {
		b, err := os.ReadFile(tFile)
		if err == nil {
			return string(b), nil
		}
	}

	b, err := embedded.Read("defaults/colima.yaml")
	if err != nil {
		return "", err
	}
	var conf config.Config
	if err := yaml.Unmarshal(b, &conf); err != nil {
		return "", err
	}
	conf.VMType = defaultVMType
	conf.MountType = startCmd.Flag("mount-type").DefValue
	b, err = yamlutil.Encode(conf)
	return string(b), err
}

var templateCmdArgs struct {
	Editor string
	Print  bool
}

func init() {
	root.Cmd().AddCommand(templateCmd)

	templateCmd.Flags().StringVar(&templateCmdArgs.Editor, "editor", "", `editor to use for edit e.g. vim, nano, code (default "$EDITOR" env var)`)
	templateCmd.Flags().BoolVar(&templateCmdArgs.Print, "print", false, `print out the configuration file path, without editing`)
}
