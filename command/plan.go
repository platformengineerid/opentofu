package command

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/terraform"
	"github.com/mitchellh/cli"
)

// PlanCommand is a Command implementation that compares a Terraform
// configuration to an actual infrastructure and shows the differences.
type PlanCommand struct {
	ContextOpts *terraform.ContextOpts
	Ui          cli.Ui
}

func (c *PlanCommand) Run(args []string) int {
	var destroy, refresh bool
	var outPath, statePath string

	cmdFlags := flag.NewFlagSet("plan", flag.ContinueOnError)
	cmdFlags.BoolVar(&destroy, "destroy", false, "destroy")
	cmdFlags.BoolVar(&refresh, "refresh", true, "refresh")
	cmdFlags.StringVar(&outPath, "out", "", "path")
	cmdFlags.StringVar(&statePath, "state", DefaultStateFilename, "path")
	cmdFlags.Usage = func() { c.Ui.Error(c.Help()) }
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	var path string
	args = cmdFlags.Args()
	if len(args) > 1 {
		c.Ui.Error(
			"The plan command expects at most one argument with the path\n" +
				"to a Terraform configuration.\n")
		cmdFlags.Usage()
		return 1
	} else if len(args) == 1 {
		path = args[0]
	} else {
		var err error
		path, err = os.Getwd()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error getting pwd: %s", err))
		}
	}

	// If the default state path doesn't exist, ignore it.
	if statePath != "" {
		if _, err := os.Stat(statePath); err != nil {
			if os.IsNotExist(err) && statePath == DefaultStateFilename {
				statePath = ""
			}
		}
	}

	// Load up the state
	var state *terraform.State
	if statePath != "" {
		f, err := os.Open(statePath)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error loading state: %s", err))
			return 1
		}

		state, err = terraform.ReadState(f)
		f.Close()
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error loading state: %s", err))
			return 1
		}
	}

	b, err := config.LoadDir(path)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error loading config: %s", err))
		return 1
	}

	c.ContextOpts.Config = b
	c.ContextOpts.Hooks = append(c.ContextOpts.Hooks, &UiHook{Ui: c.Ui})
	c.ContextOpts.State = state
	ctx := terraform.NewContext(c.ContextOpts)
	if !validateContext(ctx, c.Ui) {
		return 1
	}

	if refresh {
		c.Ui.Output("Refreshing Terraform state prior to plan...\n")
		if _, err := ctx.Refresh(); err != nil {
			c.Ui.Error(fmt.Sprintf("Error refreshing state: %s", err))
			return 1
		}
		c.Ui.Output("")
	}

	plan, err := ctx.Plan(&terraform.PlanOpts{Destroy: destroy})
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error running plan: %s", err))
		return 1
	}

	if plan.Diff.Empty() {
		c.Ui.Output(
			"No changes. Infrastructure is up-to-date. This means that Terraform\n" +
				"could not detect any differences between your configuration and\n" +
				"the real physical resources that exist. As a result, Terraform\n" +
				"doesn't need to do anything.")
		return 0
	}

	if outPath != "" {
		log.Printf("[INFO] Writing plan output to: %s", outPath)
		f, err := os.Create(outPath)
		if err == nil {
			defer f.Close()
			err = terraform.WritePlan(plan, f)
		}
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error writing plan file: %s", err))
			return 1
		}
	}

	if outPath == "" {
		c.Ui.Output(strings.TrimSpace(planHeaderNoOutput) + "\n")
	} else {
		c.Ui.Output(fmt.Sprintf(
			strings.TrimSpace(planHeaderYesOutput)+"\n",
			outPath))
	}

	c.Ui.Output(FormatPlan(plan, nil))

	return 0
}

func (c *PlanCommand) Help() string {
	helpText := `
Usage: terraform plan [options] [dir]

  Generates an execution plan for Terraform.

  This execution plan can be reviewed prior to running apply to get a
  sense for what Terraform will do. Optionally, the plan can be saved to
  a Terraform plan file, and apply can take this plan file to execute
  this plan exactly.

Options:

  -destroy            If set, a plan will be generated to destroy all resources
                      managed by the given configuration and state.

  -out=path           Write a plan file to the given path. This can be used as
                      input to the "apply" command.

  -refresh=true       Update state prior to checking for differences.

  -state=statefile    Path to a Terraform state file to use to look
                      up Terraform-managed resources. By default it will
                      use the state "terraform.tfstate" if it exists.

`
	return strings.TrimSpace(helpText)
}

func (c *PlanCommand) Synopsis() string {
	return "Generate and show an execution plan"
}

const planHeaderNoOutput = `
The Terraform execution plan has been generated and is shown below.
Resources are shown in alphabetical order for quick scanning. Green resources
will be created (or destroyed and then created if an existing resource
exists), yellow resources are being changed in-place, and red resources
will be destroyed.

Note: You didn't specify an "-out" parameter to save this plan, so when
"apply" is called, Terraform can't guarantee this is what will execute.
`

const planHeaderYesOutput = `
The Terraform execution plan has been generated and is shown below.
Resources are shown in alphabetical order for quick scanning. Green resources
will be created (or destroyed and then created if an existing resource
exists), yellow resources are being changed in-place, and red resources
will be destroyed.

Your plan was also saved to the path below. Call the "apply" subcommand
with this plan file and Terraform will exactly execute this execution
plan.

Path: %s
`
