package adb

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Device represents an Android device discovered by ADB.
type Device struct {
	Serial       string
	State        string // "device", "unauthorized", "offline"
	Product      string
	Model        string
	Manufacturer string
	Width        int
	Height       int
	TransportID  string
}

// AspectRatio returns width / height as float64 (typically ~0.45 - 0.5 in portrait).
func (d *Device) AspectRatio() float64 {
	if d.Height == 0 {
		return 9.0 / 19.5
	}
	return float64(d.Width) / float64(d.Height)
}

// DisplayName returns a clean, human-readable name for the device.
func (d *Device) DisplayName() string {
	brand := strings.Title(strings.ToLower(d.Manufacturer))
	model := d.Model
	if model == "" {
		model = d.Product
	}
	if model == "" {
		model = d.Serial
	}
	if brand != "" && !strings.HasPrefix(strings.ToLower(model), strings.ToLower(brand)) {
		return fmt.Sprintf("%s %s", brand, model)
	}
	return model
}

// ResolutionString returns formatted resolution e.g. "1080x2408".
func (d *Device) ResolutionString() string {
	if d.Width > 0 && d.Height > 0 {
		return fmt.Sprintf("%dx%d", d.Width, d.Height)
	}
	return "1080x1920"
}

// ListDevices returns all connected devices parsed from `adb devices -l`.
func (c *Client) ListDevices() ([]*Device, error) {
	out, err := c.Run("devices", "-l")
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	var devices []*Device

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		dev := &Device{
			Serial: fields[0],
			State:  fields[1],
		}

		for _, f := range fields[2:] {
			parts := strings.SplitN(f, ":", 2)
			if len(parts) == 2 {
				switch parts[0] {
				case "product":
					dev.Product = parts[1]
				case "model":
					dev.Model = parts[1]
				case "transport_id":
					dev.TransportID = parts[1]
				}
			}
		}

		devices = append(devices, dev)
	}

	return devices, nil
}

// GetFirstAuthorizedDevice finds the first device ready for casting.
func (c *Client) GetFirstAuthorizedDevice() (*Device, error) {
	devices, err := c.ListDevices()
	if err != nil {
		return nil, err
	}

	for _, dev := range devices {
		if dev.State == "device" {
			// Populate additional properties
			_ = c.PopulateDeviceInfo(dev)
			return dev, nil
		}
	}

	return nil, nil
}

// PopulateDeviceInfo fetches manufacturer, resolution, and hardware properties.
func (c *Client) PopulateDeviceInfo(dev *Device) error {
	// 1. Manufacturer
	if brand, err := c.Shell(dev.Serial, "getprop ro.product.brand"); err == nil {
		dev.Manufacturer = strings.TrimSpace(brand)
	}
	if dev.Model == "" {
		if model, err := c.Shell(dev.Serial, "getprop ro.product.model"); err == nil {
			dev.Model = strings.TrimSpace(model)
		}
	}

	// 2. Physical display size via `wm size`
	if wmSize, err := c.Shell(dev.Serial, "wm size"); err == nil {
		w, h := parseResolution(wmSize)
		if w > 0 && h > 0 {
			dev.Width = w
			dev.Height = h
		}
	}

	// Fallback standard portrait resolution if not detected
	if dev.Width == 0 || dev.Height == 0 {
		dev.Width = 1080
		dev.Height = 1920
	}

	// Ensure portrait orientation (height > width)
	if dev.Width > dev.Height {
		dev.Width, dev.Height = dev.Height, dev.Width
	}

	return nil
}

var sizeRegex = regexp.MustCompile(`(?:Physical size|Override size):\s*(\d+)x(\d+)`)

func parseResolution(output string) (int, int) {
	matches := sizeRegex.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return 0, 0
	}
	// Prefer the last match (e.g. Override size over Physical size if present)
	last := matches[len(matches)-1]
	w, _ := strconv.Atoi(last[1])
	h, _ := strconv.Atoi(last[2])
	return w, h
}
