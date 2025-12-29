package aishell

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newInstanceCmd(cfg *Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "instance",
		Short: "Print the derived instance identity for the workdir",
		Long: strings.TrimSpace(`
Print the derived instance identity for the selected workdir.

This is useful for scripting and debugging: it shows the canonicalized workdir,
the instance id (hash), and the derived container/volume names.

This command does not require Docker.
`),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			workdir, iid, container, image, volume, err := resolveInstance(cfg)
			if err != nil {
				return err
			}
			fmt.Printf("workdir:    %s\n", workdir)
			fmt.Printf("instance:   %s\n", iid)
			fmt.Printf("container:  %s\n", container)
			fmt.Printf("volume:     %s\n", volume)
			fmt.Printf("image:      %s\n", image)
			fmt.Printf("labels:\n")
			fmt.Printf("  %s=true\n", LabelManaged)
			fmt.Printf("  %s=1\n", LabelSchema)
			fmt.Printf("  %s=%s\n", LabelWorkdir, workdir)
			fmt.Printf("  %s=%s\n", LabelInstance, iid)
			fmt.Printf("  %s=%s\n", LabelVolume, volume)
			return nil
		},
	}
	return cmd
}
