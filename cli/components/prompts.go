// Package components contains small terminal interaction helpers shared by
// commands that can run interactively.
package components

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// PromptWithDefault asks for one value and returns defaultValue when the user
// submits an empty answer.
func PromptWithDefault(reader *bufio.Reader, out io.Writer, label, defaultValue string) (string, error) {
	fmt.Fprintf(out, "%s [%s]: ", label, defaultValue)
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return defaultValue, nil
	}
	return answer, nil
}
