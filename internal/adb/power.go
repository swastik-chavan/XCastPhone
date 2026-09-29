package adb

import (
	"strings"
)

// ScreenState represents the current physical display power state of the phone.
type ScreenState struct {
	IsOn        bool
	Wakefulness string
	StateString string
	Details     string
}

// CheckScreenState inspects the Android power management and display subsystem.
// It checks dumpsys power, dumpsys display, and interactive state across Android versions.
func (c *Client) CheckScreenState(serial string) (*ScreenState, error) {
	state := &ScreenState{
		IsOn: true,
	}

	// 1. Query dumpsys power
	powerOut, err := c.Shell(serial, "dumpsys power")
	if err == nil {
		for _, line := range strings.Split(powerOut, "\n") {
			line = strings.TrimSpace(line)

			// mWakefulness check
			if strings.HasPrefix(line, "mWakefulness=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					state.Wakefulness = strings.TrimSpace(parts[1])
					if state.Wakefulness == "Asleep" || state.Wakefulness == "Dozing" {
						state.IsOn = false
						state.Details = "wakefulness: " + state.Wakefulness
						return state, nil
					}
					if state.Wakefulness == "Awake" {
						state.IsOn = true
					}
				}
			}

			// Display Power: state=OFF check
			if strings.Contains(line, "Display Power: state=OFF") || strings.Contains(line, "Display Power: state=0") {
				state.IsOn = false
				state.Details = "display power off"
				return state, nil
			}

			// mInteractive / isInteractive check
			if strings.HasPrefix(line, "mInteractive=false") || strings.HasPrefix(line, "isInteractive=false") {
				state.IsOn = false
				state.Details = "interactive=false"
				return state, nil
			}
		}
	}

	// 2. Query dumpsys display for mState
	dispOut, err := c.Shell(serial, "dumpsys display")
	if err == nil {
		for _, line := range strings.Split(dispOut, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "mState=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					val := strings.TrimSpace(parts[1])
					state.StateString = val
					if strings.EqualFold(val, "OFF") || strings.EqualFold(val, "DOZE") || strings.EqualFold(val, "DOZE_SUSPEND") {
						state.IsOn = false
						state.Details = "display state: " + val
						return state, nil
					}
					if strings.EqualFold(val, "ON") {
						state.IsOn = true
					}
				}
			}
			if strings.Contains(line, "state=OFF") {
				state.IsOn = false
				state.Details = "display state=OFF"
				return state, nil
			}
		}
	}

	return state, nil
}
