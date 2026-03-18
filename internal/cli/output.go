package cli

import "fmt"

const (
	colorRed   = "\033[31m"
	colorGreen = "\033[32m"
	colorReset = "\033[0m"
)

func success(format string, a ...any) {
	fmt.Printf("%s%s%s\n", colorGreen, fmt.Sprintf(format, a...), colorReset)
}

func errMsg(format string, a ...any) {
	fmt.Printf("%sError: %s%s\n", colorRed, fmt.Sprintf(format, a...), colorReset)
}
