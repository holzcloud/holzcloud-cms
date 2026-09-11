// Command totpcode prints the current TOTP code for a secret.
//
// It exists for the browser pass QUAL-02 asks for: every admin account needs a
// second factor, and driving the admin by hand or by script needs a code that
// is valid right now. It reads the secret as an argument and prints six digits.
//
// Not built into the binary and not shipped — a tool for whoever is checking
// the application, in the same spirit as tools/i18n.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/totp"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: totpcode <secret>")
		os.Exit(2)
	}
	code, err := totp.Code(os.Args[1], time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(code)
}
