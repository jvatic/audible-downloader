package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

type Option int

const Required Option = 0

func HasRequiredOption(opts []Option) bool {
	for _, opt := range opts {
		if opt == Required {
			return true
		}
	}
	return false
}

func Radio(msg string, choices []string, opts ...Option) int {
	for {
		for i, str := range choices {
			fmt.Printf("%d) %s\n", i+1, str)
		}
		if n := Int(fmt.Sprintf("%s [1-%d, default: 1]", msg, len(choices))); n != 0 {
			if n > len(choices) || n < 1 {
				fmt.Printf("Invalid choice, please enter a number for the desired option [1-%d].\n", len(choices))
				continue
			}
			return n - 1
		} else {
			// no option given, using default
			return 0
		}
	}
}

func String(msg string, opts ...Option) string {
	required := HasRequiredOption(opts)
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s: ", msg)
		if line, err := reader.ReadString('\n'); err == nil {
			line = strings.TrimSpace(line)
			if !required || line != "" {
				return line
			}
		} else {
			break
		}
	}
	return ""
}

func Int(msg string, opts ...Option) int {
	n, err := strconv.Atoi(String(msg, opts...))
	if err != nil {
		return 0
	}
	return n
}

func Password(msg string, opts ...Option) string {
	required := HasRequiredOption(opts)
	defer fmt.Println("")
	for {
		fmt.Printf("%s: ", msg)
		if lineBytes, err := terminal.ReadPassword(int(syscall.Stdin)); err == nil {
			line := strings.TrimSpace(string(lineBytes))
			if !required || line != "" {
				return line
			}
		} else {
			break
		}
	}
	return ""
}
