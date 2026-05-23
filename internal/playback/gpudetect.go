package playback

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const defaultDRIDir = "/dev/dri"

// HWAccelInfo describes the detected hardware acceleration capability.
type HWAccelInfo struct {
	Resolved      string   `json:"resolved"`
	RenderDevices []string `json:"render_devices"`
	IntelDetected bool     `json:"intel_detected"`
	Source        string   `json:"source"`
	NodeURL       string   `json:"node_url,omitempty"`
}

// DetectHWAccel probes this host's GPU hardware and returns structured info.
func DetectHWAccel() HWAccelInfo {
	devices := listRenderDevices(defaultDRIDir)
	intel := false
	for _, d := range devices {
		if isIntelDevice(d) {
			intel = true
			break
		}
	}
	return HWAccelInfo{
		Resolved:      ResolveHWAccel("auto"),
		RenderDevices: devices,
		IntelDetected: intel,
		Source:        "local",
	}
}

// PickRenderDevice returns the GPU render device path to use.
// If explicit is non-empty, it is returned as-is.
// Otherwise, it attempts to discover a render device under /dev/dri/.
// Returns empty string if no device is found (caller should fall back to CPU).
func PickRenderDevice(explicit string) string {
	if explicit != "" {
		return explicit
	}
	dev := detectRenderDevice(defaultDRIDir)
	if dev != "" {
		slog.Info("auto-detected GPU render device", "device", dev)
	}
	return dev
}

// ResolveHWAccel resolves "auto" into a concrete acceleration method by
// probing the system. Preference order: qsv > vaapi > none.
// Non-"auto" values are returned unchanged.
func ResolveHWAccel(hwAccel string) string {
	if hwAccel != "auto" {
		return hwAccel
	}
	if runtime.GOOS != "linux" {
		return "none"
	}

	devices := listRenderDevices(defaultDRIDir)
	if len(devices) == 0 {
		slog.Info("hw_accel=auto: no render devices found, using software encoding")
		return "none"
	}

	// Check for Intel GPU — enables QSV (preferred).
	for _, dev := range devices {
		if isIntelDevice(dev) {
			slog.Info("hw_accel=auto: Intel GPU detected, using QSV", "device", dev)
			return "qsv"
		}
	}

	// Any accessible render device supports VAAPI.
	slog.Info("hw_accel=auto: non-Intel GPU detected, using VAAPI", "device", devices[0])
	return "vaapi"
}

// listRenderDevices returns all accessible /dev/dri/renderD* paths, sorted.
func listRenderDevices(driDir string) []string {
	pattern := filepath.Join(driDir, "renderD*")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Strings(matches)

	var accessible []string
	for _, dev := range matches {
		if f, err := os.Open(dev); err == nil {
			f.Close()
			accessible = append(accessible, dev)
		}
	}
	return accessible
}

// isIntelDevice checks whether a render device belongs to an Intel GPU by
// reading the PCI vendor ID from sysfs. Intel vendor ID is 0x8086.
func isIntelDevice(renderDevPath string) bool {
	// /dev/dri/renderD128 → card name "renderD128"
	name := filepath.Base(renderDevPath)
	vendorPath := filepath.Join("/sys/class/drm", name, "device", "vendor")
	data, err := os.ReadFile(vendorPath)
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "0x8086"
}

// detectRenderDevice enumerates /dev/dri/renderD* and returns the first
// available device, or empty string if none found.
func detectRenderDevice(driDir string) string {
	devices := listRenderDevices(driDir)
	if len(devices) > 0 {
		return devices[0]
	}
	return ""
}
