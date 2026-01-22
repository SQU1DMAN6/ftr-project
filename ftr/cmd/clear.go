package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os/exec"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear temporary files",
	Long:  `Remove the temporary build directory at /tmp/fsdl.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Cleaning temporary files...")
		if err := exec.Command("sudo", "rm", "-rf", "/tmp/fsdl").Run(); err != nil {
			return fmt.Errorf("failed to clean temporary directory: %w", err)
		}
		fmt.Println("Cleaning complete")
		return nil
	},
}
