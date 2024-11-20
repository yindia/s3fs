package cmd

import "github.com/spf13/cobra"

var storeCmd = &cobra.Command{
	Use:     "store",
	Aliases: []string{},
	Short:   "Store command to communicate with s3fs server",
	Long:    ``,
	Example: `  `,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	rootCmd.AddCommand(storeCmd)
}
