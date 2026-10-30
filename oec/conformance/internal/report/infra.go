package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/OI4/oec/conformance/internal/verdict"
)

// InfraVersion is the version of the JSON document of oec-infra
// (infra-report.schema.json); independent of MonitorVersion.
const InfraVersion = 1

// Infra is the report of oec-infra (D8, DESIGN.md §9): the inspected input
// and one verdict per catalog test of the mode.
type Infra struct {
	Report   int                     `json:"report"` // InfraVersion
	Tool     string                  `json:"tool"`
	Version  string                  `json:"version"`
	Schemas  string                  `json:"schemas"`
	Baseline string                  `json:"baseline"`
	Catalog  string                  `json:"catalog"`
	Command  []string                `json:"command"`
	Mode     string                  `json:"mode"` // image | bus | host
	Input    string                  `json:"input"`
	Form     string                  `json:"form,omitempty"` // image: oci-layout | oci-archive
	Level    int                     `json:"level"`
	Started  time.Time               `json:"started"`
	Ended    time.Time               `json:"ended"`
	Image    *Image                  `json:"image,omitempty"`
	Broker   *Broker                 `json:"broker,omitempty"`
	Host     *Host                   `json:"host,omitempty"`
	Tests    []verdict.TestResult    `json:"tests"`
	Summary  map[verdict.Outcome]int `json:"summary"`
	Exit     int                     `json:"exit"`
}

// Image is what oec-infra image found in the layout.
type Image struct {
	Digest     string     `json:"digest,omitempty"`
	MediaType  string     `json:"media_type,omitempty"`
	Index      bool       `json:"index"`
	Manifests  []Manifest `json:"manifests"`
	Algorithms []string   `json:"algorithms,omitempty"`
	Problems   int        `json:"problems,omitempty"`
}

// Manifest is one image manifest of the layout.
type Manifest struct {
	Digest    string `json:"digest"`
	MediaType string `json:"media_type"`
	Platform  string `json:"platform,omitempty"`
	Runnable  bool   `json:"runnable"`
	Supported bool   `json:"supported"`
	Layers    int    `json:"layers,omitempty"`
}

// Broker is what oec-infra bus found on the Message Bus.
type Broker struct {
	URL           string       `json:"url"`
	Probe         string       `json:"probe"`                     // Identification Link of the probe AppId of the run
	TLS           []string     `json:"tls,omitempty"`             // TLS versions the broker accepted (1.2, 1.3)
	Certificate   *Certificate `json:"certificate,omitempty"`     // the broker certificate
	MaxPacketSize int          `json:"max_packet_size,omitempty"` // the probed MaxPacketSize
	Registry      string       `json:"registry,omitempty"`        // Identification Link of the OEC Registry
}

// Certificate describes the broker certificate.
type Certificate struct {
	Subject   string    `json:"subject"`
	Issuer    string    `json:"issuer"`
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	Names     []string  `json:"names,omitempty"` // DNS names and IP addresses of the subject alternative name
}

// Host is what oec-infra host found in the storages and on the platform
// host: names, modes and sizes, never a file content.
type Host struct {
	Root      string    `json:"root"`
	Container string    `json:"container,omitempty"`
	Storages  []Storage `json:"storages"`
	Platform  *Platform `json:"platform,omitempty"`
}

// Storage is one container storage of RIG 4.1.
type Storage struct {
	Name   string `json:"name"` // mqtt | certs | secrets
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Mount  string `json:"mount"` // ro | rw | none | unknown
	Files  []File `json:"files,omitempty"`
}

// File is one entry of a storage.
type File struct {
	Name string `json:"name"`
	Mode string `json:"mode,omitempty"`
	Size int64  `json:"size"`
}

// Platform is what oec-infra host measured on the platform host (RIG 5.1).
type Platform struct {
	OS            string   `json:"os,omitempty"`
	Kernel        string   `json:"kernel,omitempty"`
	Distribution  string   `json:"distribution,omitempty"`
	Machine       string   `json:"machine,omitempty"`
	Memory        uint64   `json:"memory,omitempty"`
	Storage       uint64   `json:"storage,omitempty"`
	StorageSource string   `json:"storage_source,omitempty"`
	Interfaces    []string `json:"interfaces,omitempty"`
	Daemon        string   `json:"daemon,omitempty"`
	Broker        string   `json:"broker,omitempty"`
	Registry      string   `json:"registry,omitempty"`
}

// JSON writes the report as one JSON document.
func (r *Infra) JSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r)
}

// Text writes the input, one line per test and the evidence of every
// requirement that is not fulfilled.
func (r *Infra) Text(w io.Writer) error {
	var b strings.Builder
	if r.Form != "" {
		fmt.Fprintf(&b, "%s %s %s (%s %s), level %d, schemas %s, baseline %s\n", r.Tool, r.Version, r.Mode, r.Form, r.Input, r.Level, r.Schemas, r.Baseline)
	} else {
		fmt.Fprintf(&b, "%s %s %s (%s), level %d, schemas %s, baseline %s\n", r.Tool, r.Version, r.Mode, r.Input, r.Level, r.Schemas, r.Baseline)
	}
	if br := r.Broker; br != nil {
		fmt.Fprintf(&b, "probe AppId %s\n", br.Probe)
		if len(br.TLS) > 0 {
			fmt.Fprintf(&b, "TLS %s\n", strings.Join(br.TLS, ", "))
		}
		if c := br.Certificate; c != nil {
			fmt.Fprintf(&b, "certificate %q issued by %q, valid %s … %s", c.Subject, c.Issuer, c.NotBefore.Format(time.DateOnly), c.NotAfter.Format(time.DateOnly))
			if len(c.Names) > 0 {
				fmt.Fprintf(&b, ", names %s", strings.Join(c.Names, ", "))
			}
			b.WriteString("\n")
		}
		if br.MaxPacketSize > 0 {
			fmt.Fprintf(&b, "MaxPacketSize %d\n", br.MaxPacketSize)
		}
		if br.Registry != "" {
			fmt.Fprintf(&b, "OEC Registry %s\n", br.Registry)
		}
	}
	if h := r.Host; h != nil {
		writeHost(&b, h)
	}
	if im := r.Image; im != nil {
		kind := "image manifest"
		if im.Index {
			kind = "image index"
		}
		if im.Digest != "" {
			fmt.Fprintf(&b, "%s %s\n", kind, im.Digest)
		} else {
			fmt.Fprintf(&b, "%s (%d descriptors in index.json)\n", kind, len(im.Manifests))
		}
		for _, m := range im.Manifests {
			plat := m.Platform
			if plat == "" {
				plat = "unknown platform"
			}
			fmt.Fprintf(&b, "  %s %s", plat, m.Digest)
			switch {
			case !m.Runnable:
				b.WriteString(" (not runnable)")
			case !m.Supported:
				b.WriteString(" (platform not supported)")
			}
			b.WriteString("\n")
		}
		if im.Problems > 0 {
			fmt.Fprintf(&b, "%d structure or integrity problem(s) while reading\n", im.Problems)
		}
	}
	b.WriteString("tests:\n")
	writeTests(&b, r.Tests)
	fmt.Fprintf(&b, "summary: %s; result %s (exit %d)\n", summaryLine(r.Summary), exitText[r.Exit], r.Exit)
	_, err := io.WriteString(w, b.String())
	return err
}

func writeHost(b *strings.Builder, h *Host) {
	if h.Container != "" {
		fmt.Fprintf(b, "container %s\n", h.Container)
	}
	for _, s := range h.Storages {
		if !s.Exists {
			fmt.Fprintf(b, "storage %s %s: missing\n", s.Name, s.Path)
			continue
		}
		names := make([]string, len(s.Files))
		for i, f := range s.Files {
			names[i] = f.Name
			if f.Mode != "" {
				names[i] += " " + f.Mode
			}
		}
		fmt.Fprintf(b, "storage %s %s (mount %s): %s\n", s.Name, s.Path, s.Mount, strings.Join(names, ", "))
	}
	p := h.Platform
	if p == nil {
		return
	}
	if p.OS != "" {
		fmt.Fprintf(b, "platform %s %s %s", p.OS, p.Machine, p.Kernel)
		if p.Distribution != "" {
			fmt.Fprintf(b, " (%s)", p.Distribution)
		}
		b.WriteString("\n")
	}
	if p.Memory > 0 {
		fmt.Fprintf(b, "memory %d octets\n", p.Memory)
	}
	if p.Storage > 0 {
		fmt.Fprintf(b, "non-volatile memory %d octets (%s)\n", p.Storage, p.StorageSource)
	}
	if len(p.Interfaces) > 0 {
		fmt.Fprintf(b, "network interfaces %s\n", strings.Join(p.Interfaces, ", "))
	}
	if p.Daemon != "" {
		fmt.Fprintf(b, "container daemon %s\n", p.Daemon)
	}
	if p.Broker != "" {
		fmt.Fprintf(b, "broker %s\n", p.Broker)
	}
	if p.Registry != "" {
		fmt.Fprintf(b, "OEC Registry %s\n", p.Registry)
	}
}

// JUnit writes the report as JUnit XML (same structure as oec-check).
func (r *Infra) JUnit(w io.Writer) error {
	return writeJUnit(w, r.Tool, r.Started, []junitProperty{{"level", fmt.Sprint(r.Level)}, {"mode", r.Mode}, {"schemas", r.Schemas}, {"baseline", r.Baseline}}, r.Tests)
}
