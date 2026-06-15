package cmd

import (
	"fmt"
	"io"
)

// cincBanner is the "CINC" wordmark (figlet "ANSI Shadow") shown above the
// first-run welcome prompts.
const cincBanner = ` ██████╗██╗███╗   ██╗ ██████╗
██╔════╝██║████╗  ██║██╔════╝
██║     ██║██╔██╗ ██║██║
██║     ██║██║╚██╗██║██║
╚██████╗██║██║ ╚████║╚██████╗
 ╚═════╝╚═╝╚═╝  ╚═══╝ ╚═════╝`

// writeWelcomeBanner prints the first-run welcome headline: a "Welcome to"
// lead-in above the CINC wordmark.
func writeWelcomeBanner(w io.Writer) {
	fmt.Fprintln(w, "Welcome to")
	fmt.Fprintln(w)
	fmt.Fprintln(w, cincBanner)
}
