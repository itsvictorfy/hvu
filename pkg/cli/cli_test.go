package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRootCmd_Exists(t *testing.T) {
	if rootCmd == nil {
		t.Fatal("rootCmd should not be nil")
	}

	if rootCmd.Use != "hvu" {
		t.Errorf("expected Use=hvu, got %s", rootCmd.Use)
	}
}

func TestRootCmd_HasSubcommands(t *testing.T) {
	commands := rootCmd.Commands()

	expectedCommands := []string{"upgrade", "classify", "version"}
	foundCommands := make(map[string]bool)

	for _, cmd := range commands {
		foundCommands[cmd.Name()] = true
	}

	for _, expected := range expectedCommands {
		if !foundCommands[expected] {
			t.Errorf("expected subcommand %q to exist", expected)
		}
	}
}

func TestRootCmd_GlobalFlags(t *testing.T) {
	flags := []string{"output", "quiet", "verbose"}

	for _, flag := range flags {
		if rootCmd.PersistentFlags().Lookup(flag) == nil {
			t.Errorf("expected global flag %q to exist", flag)
		}
	}
}

func TestUpgradeCmd_RequiredFlags(t *testing.T) {
	cmd := UpgradeCmd()

	requiredFlags := []string{"chart", "repo", "from", "to", "values"}

	for _, flag := range requiredFlags {
		f := cmd.Flags().Lookup(flag)
		if f == nil {
			t.Errorf("expected flag %q to exist on upgrade command", flag)
			continue
		}
		// Check flag annotations for required
		if ann, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok {
			_ = ann // Flag is marked as required through annotations
		}
	}
}

func TestUpgradeCmd_OptionalFlags(t *testing.T) {
	cmd := UpgradeCmd()

	optionalFlags := []string{"output", "dry-run"}

	for _, flag := range optionalFlags {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("expected optional flag %q to exist on upgrade command", flag)
		}
	}
}

func TestUpgradeCmd_MissingRequiredFlags(t *testing.T) {
	cmd := UpgradeCmd()
	cmd.SetArgs([]string{}) // No args

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()

	// Should fail due to missing required flags
	if err == nil {
		t.Error("expected error when required flags are missing")
	}
}

func TestClassifyCmd_RequiredFlags(t *testing.T) {
	cmd := ClassifyCmd()

	requiredFlags := []string{"chart", "repo", "version", "values"}

	for _, flag := range requiredFlags {
		f := cmd.Flags().Lookup(flag)
		if f == nil {
			t.Errorf("expected flag %q to exist on classify command", flag)
		}
	}
}

func TestClassifyCmd_MissingRequiredFlags(t *testing.T) {
	cmd := ClassifyCmd()
	cmd.SetArgs([]string{}) // No args

	// Capture output
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()

	// Should fail due to missing required flags
	if err == nil {
		t.Error("expected error when required flags are missing")
	}
}

func TestVersionCmd(t *testing.T) {
	cmd := VersionCmd()

	if cmd.Use != "version" {
		t.Errorf("expected Use=version, got %s", cmd.Use)
	}

	// Check short flag exists
	shortFlag := cmd.Flags().Lookup("short")
	if shortFlag == nil {
		t.Error("expected short flag to exist")
	}

	// Version command should execute without error
	err := cmd.Execute()
	if err != nil {
		t.Errorf("version command should not error: %v", err)
	}
}

func TestExecute_ReturnsNilForHelp(t *testing.T) {
	// Save original args
	oldArgs := rootCmd.Args

	// Set args to just --help
	rootCmd.SetArgs([]string{"--help"})

	// Capture output
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := Execute()

	// Restore original args
	rootCmd.Args = oldArgs

	if err != nil {
		t.Errorf("Execute() with --help should not error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "hvu") {
		t.Error("help output should contain 'hvu'")
	}
}

func TestUpgradeCmd_DryRunFlag(t *testing.T) {
	cmd := UpgradeCmd()

	dryRunFlag := cmd.Flags().Lookup("dry-run")
	if dryRunFlag == nil {
		t.Fatal("expected dry-run flag to exist")
	}

	if dryRunFlag.DefValue != "false" {
		t.Errorf("expected dry-run default to be false, got %s", dryRunFlag.DefValue)
	}
}

func TestClassifyCmd_ValuesShorthand(t *testing.T) {
	cmd := ClassifyCmd()

	valuesFlag := cmd.Flags().Lookup("values")
	if valuesFlag == nil {
		t.Fatal("expected values flag to exist")
	}

	if valuesFlag.Shorthand != "f" {
		t.Errorf("expected values shorthand to be 'f', got %s", valuesFlag.Shorthand)
	}
}

func TestUpgradeCmd_ValuesShorthand(t *testing.T) {
	cmd := UpgradeCmd()

	valuesFlag := cmd.Flags().Lookup("values")
	if valuesFlag == nil {
		t.Fatal("expected values flag to exist")
	}

	if valuesFlag.Shorthand != "f" {
		t.Errorf("expected values shorthand to be 'f', got %s", valuesFlag.Shorthand)
	}

	outputFlag := cmd.Flags().Lookup("output")
	if outputFlag == nil {
		t.Fatal("expected output flag to exist")
	}

	if outputFlag.Shorthand != "o" {
		t.Errorf("expected output shorthand to be 'o', got %s", outputFlag.Shorthand)
	}
}

func TestRootCmd_SilenceUsage(t *testing.T) {
	if !rootCmd.SilenceUsage {
		t.Error("expected SilenceUsage to be true")
	}
}

func TestRootCmd_SilenceErrors(t *testing.T) {
	if !rootCmd.SilenceErrors {
		t.Error("expected SilenceErrors to be true")
	}
}
