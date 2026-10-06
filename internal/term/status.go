package term

import (
	"fmt"
	"os"
)

const reset = "\033[0m"

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

// StatusBadge returns a colored bracket badge for up/down status.
func StatusBadge(up bool) string {
	if !colorEnabled() {
		if up {
			return "[ up ]"
		}
		return "[ down ]"
	}

	if up {
		// Bright green foreground
		return fmt.Sprintf("\033[1;92m[ up ]%s", reset)
	}
	// Bright red foreground — unmistakably red across terminals
	return fmt.Sprintf("\033[1;91m[ down ]%s", reset)
}
