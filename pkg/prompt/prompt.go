package prompt

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/itsvictorfy/hvu/pkg/values"
)

// Prompter defines the interface for user prompts
type Prompter interface {
	ConfirmImageUpgrade(changes []values.ImageChange) (bool, error)
}

// InteractivePrompter prompts users via stdin/stdout
type InteractivePrompter struct {
	reader io.Reader
	writer io.Writer
}

// NewInteractivePrompter creates a new prompter using stdin/stdout
func NewInteractivePrompter() *InteractivePrompter {
	return &InteractivePrompter{
		reader: os.Stdin,
		writer: os.Stdout,
	}
}

// NewPrompterWithIO creates a prompter with custom IO (useful for testing)
func NewPrompterWithIO(reader io.Reader, writer io.Writer) *InteractivePrompter {
	return &InteractivePrompter{
		reader: reader,
		writer: writer,
	}
}

// ConfirmImageUpgrade displays custom image tags and asks user if they want to upgrade them
func (p *InteractivePrompter) ConfirmImageUpgrade(changes []values.ImageChange) (bool, error) {
	if len(changes) == 0 {
		return false, nil
	}

	fmt.Fprintln(p.writer)
	fmt.Fprintln(p.writer, "Custom image tags detected:")
	fmt.Fprintln(p.writer)

	for _, change := range changes {
		displayPath := values.PathToDisplayFormat(change.Path)
		fmt.Fprintf(p.writer, "  %s:\n", displayPath)
		fmt.Fprintf(p.writer, "    Current:     %s\n", change.UserTag)
		fmt.Fprintf(p.writer, "    Old default: %s\n", change.OldDefault)
		fmt.Fprintf(p.writer, "    New default: %s\n", change.NewDefault)
		fmt.Fprintln(p.writer)
	}

	fmt.Fprint(p.writer, "Would you like to upgrade image tags to new defaults? [y/N]: ")

	scanner := bufio.NewScanner(p.reader)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return false, fmt.Errorf("failed to read input: %w", err)
		}
		return false, nil
	}

	response := strings.TrimSpace(strings.ToLower(scanner.Text()))
	return response == "y" || response == "yes", nil
}
