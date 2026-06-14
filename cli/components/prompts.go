// Package components contains small terminal interaction helpers shared by
// commands that can run interactively.
package components

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// PromptPassword reads a secret from in, without echoing it to the terminal
// when in is an interactive TTY. For non-terminal input (a pipe, redirect, or a
// test) it falls back to reading a single newline-terminated line. label is
// shown as the prompt.
func PromptPassword(in io.Reader, out io.Writer, label string) (string, error) {
	fmt.Fprintf(out, "%s: ", label)
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(out) // ReadPassword consumes the Enter without a newline
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

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
