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
	Report    int                     `json:"report"` // InfraVersion
	Tool      string                  `json:"tool"`
	Version   string                  `json:"version"`
	Schemas   string                  `json:"schemas"`
	Baseline  string                  `json:"baseline"`
	Catalog   string                  `json:"catalog"`
	Command   []string                `json:"command"`
	Mode      string                  `json:"mode"` // image | bus | host | container | run
	Input     string                  `json:"input"`
	Form      string                  `json:"form,omitempty"` // image: oci-layout | oci-archive
	Level     int                     `json:"level"`
	Started   time.Time               `json:"started"`
	Ended     time.Time               `json:"ended"`
	Image     *Image                  `json:"image,omitempty"`
	Broker    *Broker                 `json:"broker,omitempty"`
	Host      *Host                   `json:"host,omitempty"`
	Container *ContainerRun           `json:"container,omitempty"`
	Run       *Run                    `json:"run,omitempty"`
	Tests     []verdict.TestResult    `json:"tests"`
	Summary   map[verdict.Outcome]int `json:"summary"`
	Exit      int                     `json:"exit"`
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
	// Engine and Containers: T8-HST-03 (C-073), the OI4 containers of the
	// engine except the one D8 runs in.
	Engine     *Engine     `json:"engine,omitempty"`
	Containers []Container `json:"containers,omitempty"`
}

// ContainerRun is what oec-infra container read through the Engine API
// (C-073): no environment, no label, no address and no mount source.
type ContainerRun struct {
	Engine     Engine      `json:"engine"`
	Containers []Container `json:"containers"`
}

// Run is what oec-infra run started and observed (C-074): no secret, no
// username, no log, no environment and no host path.
type Run struct {
	Engine   Engine      `json:"engine"`
	Image    RunImage    `json:"image"`
	Endpoint RunEndpoint `json:"endpoint"`
	Cleanup  string      `json:"cleanup,omitempty"`
	Phases   []RunPhase  `json:"phases"`
}

// RunImage is the image of the run.
type RunImage struct {
	Ref  string `json:"ref"`
	ID   string `json:"id,omitempty"`
	OS   string `json:"os,omitempty"`
	Arch string `json:"arch,omitempty"`
}

// RunEndpoint is the MQTT test endpoint of D8.
type RunEndpoint struct {
	Listen        string `json:"listen"`
	BrokerHost    string `json:"broker_host"`
	MaxPacketSize int    `json:"max_packet_size"`
}

// RunPhase is one container of the run.
type RunPhase struct {
	Name        string          `json:"name"` // credentials | certificate | untrusted
	Container   string          `json:"container,omitempty"`
	Port        int             `json:"port,omitempty"`
	Started     bool            `json:"started"`
	Running     bool            `json:"running"`
	ExitCode    *int            `json:"exit_code,omitempty"`
	ObservedMS  int64           `json:"observed_ms"`
	Removed     bool            `json:"removed"`
	Error       string          `json:"error,omitempty"`
	Connections []RunConnection `json:"connections"`
	Birth       *Birth          `json:"birth,omitempty"`
}

// RunConnection is one TLS connection of the container to the endpoint.
type RunConnection struct {
	Handshake         string `json:"handshake"` // ok | failed
	HandshakeError    string `json:"handshake_error,omitempty"`
	TLS               string `json:"tls,omitempty"`      // 1.2 | 1.3
	ClientCertificate string `json:"client_certificate"` // none | verified | untrusted
	ProtocolLevel     int    `json:"protocol_level,omitempty"`
	ClientID          string `json:"client_id,omitempty"`
	Auth              string `json:"auth,omitempty"` // credentials | credentials-wrong | certificate | none
	Accepted          bool   `json:"accepted"`
	Publishes         int    `json:"publishes"`
	LargestPayload    int    `json:"largest_payload"`
	Oversized         int    `json:"oversized"`
}

// Birth is the first MAM of the application about itself.
type Birth struct {
	Topic              string `json:"topic"`
	IdentificationLink string `json:"identification_link"`
	SerialNumber       string `json:"serial_number,omitempty"`
}

// Engine is a Docker or Podman Engine API.
type Engine struct {
	Endpoint string `json:"endpoint"`
	Product  string `json:"product,omitempty"`
	Version  string `json:"version,omitempty"`
	API      string `json:"api,omitempty"`
	OS       string `json:"os,omitempty"`
	Arch     string `json:"arch,omitempty"`
}

// Container is one inspected container.
type Container struct {
	ID          string           `json:"id,omitempty"` // short id
	Name        string           `json:"name"`
	Hostname    string           `json:"hostname,omitempty"`
	Image       string           `json:"image,omitempty"`
	Running     bool             `json:"running"`
	NetworkMode string           `json:"network_mode,omitempty"`
	Networks    []Network        `json:"networks,omitempty"`
	Mounts      []ContainerMount `json:"mounts"`
}

// Network is a network a container is attached to.
type Network struct {
	Name   string `json:"name"`
	Driver string `json:"driver,omitempty"`
}

// ContainerMount is a mount of a container without its source.
type ContainerMount struct {
	Destination string `json:"destination"`
	Type        string `json:"type"`
	Mode        string `json:"mode"` // ro | rw
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
	if c := r.Container; c != nil {
		writeEngine(&b, &c.Engine)
		for _, ct := range c.Containers {
			writeContainer(&b, ct)
		}
	}
	if rn := r.Run; rn != nil {
		writeRun(&b, rn)
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
	if h.Engine != nil {
		writeEngine(b, h.Engine)
	}
	for _, c := range h.Containers {
		writeContainer(b, c)
	}
}

func writeRun(b *strings.Builder, r *Run) {
	writeEngine(b, &r.Engine)
	fmt.Fprintf(b, "image %s", r.Image.Ref)
	if r.Image.ID != "" {
		fmt.Fprintf(b, " (%s, %s/%s)", r.Image.ID, r.Image.OS, r.Image.Arch)
	}
	b.WriteString("\n")
	fmt.Fprintf(b, "MQTT test endpoint on %s, broker address %s, MaxPacketSize %d\n", r.Endpoint.Listen, r.Endpoint.BrokerHost, r.Endpoint.MaxPacketSize)
	for _, p := range r.Phases {
		fmt.Fprintf(b, "phase %s: container %s, port %d", p.Name, p.Container, p.Port)
		switch {
		case p.Error != "":
			fmt.Fprintf(b, ", %s", p.Error)
		case !p.Started:
			b.WriteString(", not started")
		case p.Running:
			fmt.Fprintf(b, ", ran %.1f s", float64(p.ObservedMS)/1000)
		case p.ExitCode != nil:
			fmt.Fprintf(b, ", exited with %d after %.1f s", *p.ExitCode, float64(p.ObservedMS)/1000)
		}
		if !p.Removed {
			b.WriteString(", NOT removed")
		}
		b.WriteString("\n")
		for _, c := range p.Connections {
			if c.Handshake != "ok" {
				fmt.Fprintf(b, "  connection: TLS handshake failed (%s)\n", c.HandshakeError)
				continue
			}
			fmt.Fprintf(b, "  connection: TLS %s, client certificate %s", c.TLS, c.ClientCertificate)
			if c.ProtocolLevel > 0 {
				acc := "refused"
				if c.Accepted {
					acc = "accepted"
				}
				fmt.Fprintf(b, ", CONNECT level %d ClientId %q auth %s %s, %d PUBLISH (largest payload %d octets, %d oversized)", c.ProtocolLevel, c.ClientID, c.Auth, acc, c.Publishes, c.LargestPayload, c.Oversized)
			} else {
				b.WriteString(", no CONNECT")
			}
			b.WriteString("\n")
		}
		if p.Birth != nil {
			fmt.Fprintf(b, "  birth %s (%s)\n", p.Birth.Topic, p.Birth.IdentificationLink)
		}
	}
	if r.Cleanup != "" {
		fmt.Fprintf(b, "cleanup: %s\n", r.Cleanup)
	}
}

func writeEngine(b *strings.Builder, e *Engine) {
	fmt.Fprintf(b, "engine %s", e.Endpoint)
	if e.Product != "" {
		fmt.Fprintf(b, ": %s %s (API %s, %s/%s)", e.Product, e.Version, e.API, e.OS, e.Arch)
	}
	b.WriteString("\n")
}

func writeContainer(b *strings.Builder, c Container) {
	fmt.Fprintf(b, "container %s", c.Name)
	var at []string
	if c.ID != "" {
		at = append(at, c.ID)
	}
	if c.Image != "" {
		at = append(at, c.Image)
	}
	if !c.Running {
		at = append(at, "not running")
	}
	if len(at) > 0 {
		fmt.Fprintf(b, " (%s)", strings.Join(at, ", "))
	}
	b.WriteString("\n")
	if c.Hostname != "" {
		fmt.Fprintf(b, "  hostname %s\n", c.Hostname)
	}
	if c.NetworkMode != "" {
		nets := make([]string, len(c.Networks))
		for i, n := range c.Networks {
			nets[i] = n.Name
			if n.Driver != "" {
				nets[i] += " (" + n.Driver + ")"
			}
		}
		fmt.Fprintf(b, "  network mode %s", c.NetworkMode)
		if len(nets) > 0 {
			fmt.Fprintf(b, ", networks %s", strings.Join(nets, ", "))
		}
		b.WriteString("\n")
	}
	for _, m := range c.Mounts {
		fmt.Fprintf(b, "  mount %s %s %s\n", m.Destination, m.Type, m.Mode)
	}
}

// JUnit writes the report as JUnit XML (same structure as oec-check).
func (r *Infra) JUnit(w io.Writer) error {
	return writeJUnit(w, r.Tool, r.Started, []junitProperty{{"level", fmt.Sprint(r.Level)}, {"mode", r.Mode}, {"schemas", r.Schemas}, {"baseline", r.Baseline}}, r.Tests)
}
