// Command oec-infra is the infrastructure compliance CLI of OEC v2.0
// (deliverable D8, DESIGN.md §9). The command image inspects an OCI image
// offline – an OCI image layout directory or an oci-archive – and judges the
// tests of mode image of the D8 catalog (T8-IMG-01 … 04): OCI format,
// supported platforms, SBOM and license texts, digest integrity. No
// container daemon and no network are needed. The command bus probes a
// running Message Bus and judges the tests of mode bus (T8-BUS-01 … 03):
// protocol, TLS and certificate, MaxPacketSize, ACL, OEC Registry. The
// command host reads the storages of RIG 4.1 below a root directory and
// measures the platform host it runs on (T8-HST-01, 02, 04): Message Bus
// storage and broker.json, certificate and secret storages, hardware,
// Linux, container daemon, Message Bus and Registry. The command container
// reads containers through the Docker/Podman Engine API and judges the tests
// of mode container (T8-CTR-01 … 03): networks, container name and
// hostname, no host mount on /opt/oi4/licenses. The command run starts the
// application image in containers of its own through the Engine API, with
// generated storages and an MQTT test endpoint of D8, and judges the tests
// of mode run (T8-RUN-01 … 03): Message Bus storage, certificate and secret
// storages, container name as hostname and identity. list and explain show
// the tests of the D8 catalog and the requirements with the expected
// verdicts of the self-test fixtures. report-schema prints the JSON Schema
// of the report (infra-report.schema.json, report version 1, frozen at the
// P5b exit).
//
// Exit codes (DESIGN.md §5.3): 0 pass (warnings allowed), 1 at least one
// requirement violated, 2 usage or input error (also: the input is no OCI
// image layout, the engine knows no such container or image, the engine is
// remote), 3 internal error, 4
// inconclusive (a mandatory requirement could not be judged; --allow-skip
// turns it into 0).
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/OI4/oec/conformance"
	"github.com/OI4/oec/conformance/catalog"
	"github.com/OI4/oec/conformance/internal/image"
	"github.com/OI4/oec/conformance/internal/oci"
	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/topic"
	"github.com/OI4/oec/conformance/internal/verdict"
	schema "github.com/OI4/oec/schema"
)

const tool = "oec-infra"

// version is set by the release build (-ldflags "-X main.version=…").
var version = "dev"

const (
	exitPass         = 0
	exitUsage        = 2
	exitInternal     = 3
	exitInconclusive = 4
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

const usage = `usage:
  oec-infra image [flags] OCI-LAYOUT-DIR | OCI-ARCHIVE.tar
  oec-infra bus [flags]
  oec-infra host [flags]
  oec-infra container [flags] CONTAINER...
  oec-infra run [flags] IMAGE
  oec-infra list [--mode image|run|container|host|bus] [--level 1|2|3]
                 [--req REQ-…] [--needs docker|host-access|credentials-2|none]
                 [--status planned|implemented] [--format text|json] [ID|GLOB ...]
  oec-infra explain [--format text|json] T8-…|REQ-…
  oec-infra report-schema
  oec-infra version

image inspects an OCI image offline and judges the image tests of the D8
catalog (T8-IMG-01 … 04): OCI image format, supported platforms (linux on
amd64, arm64/v8, arm/v7), SBOM in /opt/oi4/licenses with a text for every
license, SHA256 digests. The input is an OCI image layout directory or an
uncompressed tar of it (oci-archive), e.g. from
  docker save IMAGE -o image.tar        (Docker 25 or later)
  skopeo copy docker://REF oci-archive:image.tar
  podman save --format oci-archive IMAGE -o image.tar
A registry reference and a Docker archive without index.json are not read.
  verdict:    --level 1|2|3 (default 3), --allow-skip
  output:     --format text|json|junit, --report FORMAT=PATH (repeatable)

bus probes a running Message Bus like an OEC application and judges the
bus tests of the D8 catalog (T8-BUS-01 … 03) at level 3: MQTT 3.1.1, TLS 1.2
or higher without a plain-text listener, the broker certificate (chain to
the CA, host name, validity), MaxPacketSize; the ACL against a second,
unprivileged client (Set, Del and Call must not arrive, Set/Config should
not be readable); the OEC Registry (collects MAM and Health, answers Get).
It publishes only on the topics of its own probe AppId
(Utility/oi4.org/oec-infra/D8/<random>).
  connection: --broker mqtts://host[:port] | --bus-storage ROOT, --ca FILE,
              --cert FILE --key FILE [--key-passphrase-file FILE],
              --username NAME --password-file FILE, --insecure (dev only),
              --config FILE (YAML, keys = flag names)
  ACL probe:  --credentials-2 FILE | --bus-storage-2 ROOT (unprivileged
              client; without it T8-BUS-02 is inconclusive)
  probes:     --plain-port N (repeatable, default 1883, 0: none),
              --max-packet-size N (default broker.json, else 262144),
              --max-probe-size N (default 64 MiB), --timeout D (default 5s)
  Registry:   --registry IL (default: discovered), --registry-wait D
              (default 65s)
  verdict:    --tests IDS, --allow-skip
  output:     --format text|json|junit, --report FORMAT=PATH (repeatable)

host reads the storages of RIG 4.1 below a root directory and measures the
platform host it runs on; it judges the host tests of the D8 catalog
(T8-HST-01 … 04) at level 3: the Message Bus storage (broker.pem,
broker.json against platform/broker.schema.json, broker_ca.pem), the OI4
certificate storage (ca.pem, <containername>.pem) and the secret storage
(credentials, private_key.pem, passphrase), each mounted read-only (Linux
mount table), no private key outside the secret storage; architecture,
main and non-volatile memory, network interface, Linux (the patch level is
reported for a manual verdict), container daemon; it connects to the broker
of broker.json with the login of the storages and waits for the OEC
Registry; through the Engine API, no other container reaches the secret
storage or – with a persistent layer – the application storages
(/etc/oi4/app, /opt/oi4/app) of an OI4 container. It never writes and
never prints a secret. Run it inside the application container, or on the
host with --root pointing to the files of the container (mounts are then
not judged).
  storages:   --root DIR (default /), --mqtt-storage DIR, --certs-storage DIR,
              --secrets-storage DIR, --container NAME (default
              ROOT/etc/hostname), --config FILE (YAML, keys = flag names)
  platform:   --daemon ENDPOINT (default DOCKER_HOST, CONTAINER_HOST, the
              default sockets), --self CONTAINER (the container D8 runs
              in; T8-HST-03 ignores it), --offline (no Message Bus),
              --registry-wait D (default 65s), --timeout D (default 5s)
  verdict:    --tests IDS, --allow-skip
  output:     --format text|json|junit, --report FORMAT=PATH (repeatable)

container reads one or more containers (name, id or unique id prefix)
through the Docker/Podman Engine API and judges the container tests of the
D8 catalog (T8-CTR-01 … 03): the network mode and the drivers of the
attached networks (a bridge network, RIG 4.4), the container name (at most
63 characters, RFC 1123, unique, <DefaultContainerName>-<uuid>) set as
hostname, and no bind mount or tmpfs of the host on /opt/oi4/licenses. It
only reads (GET) and never reports the environment, labels or mount
sources of a container. An unknown container is a usage error.
  engine:     --daemon ENDPOINT (default DOCKER_HOST, CONTAINER_HOST, the
              default sockets), --timeout D (default 10s),
              --config FILE (YAML, keys = flag names)
  verdict:    --level 1|2|3 (default 3), --tests IDS, --allow-skip
  output:     --format text|json|junit, --report FORMAT=PATH (repeatable)

run starts the application image (name, id or unique id prefix of an image
the engine already has; it never pulls) in sandbox containers of its own
through the Docker/Podman Engine API on this machine and judges the run
tests of the D8 catalog (T8-RUN-01 … 03): it generates the Message Bus
storage (broker.json with a non-default SecurePort, MaxPacketSize and an
unknown element, broker.pem, ca.pem), the certificate storage and the
secret storage (credentials or client certificate with an encrypted key)
of RIG 4.1, names each container oec-d8-run-<uuid> and sets the name as
hostname (RIG 4.2), and runs an MQTT test endpoint (TLS, credentials or
client certificate) that answers like a broker without forwarding. Three
containers: credentials, certificate and an untrusted broker CA (the
application must not connect). It checks the connection to Address and
SecurePort, the login, the payload sizes, the client id and the Birth
message (MAM of the application with the container name as SerialNumber).
It creates, starts, stops and removes only containers named oec-d8-run-…
with its labels, never volumes, networks or images, and never reports a
secret, a username, a log or a host path.
  engine:     --daemon ENDPOINT (local only; default DOCKER_HOST,
              CONTAINER_HOST, the default sockets), --timeout D (default
              10s), --config FILE (YAML, keys = flag names)
  sandbox:    --wait D (default 30s, until the Birth message), --settle D
              (default 2s), --listen ADDR (default 127.0.0.1 on macOS and
              Windows, the bridge gateway elsewhere), --broker-host NAME|IP
              (default oec-d8-broker via host-gateway), --max-packet-size N
              (default 262144)
  verdict:    --level 1|2|3 (default 3), --tests IDS, --allow-skip
  output:     --format text|json|junit, --report FORMAT=PATH (repeatable)

list shows the tests of the D8 catalog with mode, level ("-": not graded,
the requirements apply with their keyword), needs, status and requirements;
explain shows a test with its requirements, or a requirement (statement,
source in the Runtime & Infrastructure Guideline (RIG) or the Messaging
Guideline (MG), the tests that check it), with the expected verdicts of the
self-test fixtures. report-schema prints the JSON Schema of the JSON report
(report version 1).

Exit codes: 0 pass, 1 fail, 2 usage or input error (no OCI image layout,
no such container or image, a remote engine), 3 internal error, 4
inconclusive.
`

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "image":
		return runImage(args[1:], stdout, stderr)
	case "bus":
		return runBus(args[1:], stdout, stderr)
	case "host":
		return runHost(args[1:], stdout, stderr)
	case "container":
		return runContainer(args[1:], stdout, stderr)
	case "run":
		return runRun(args[1:], stdout, stderr)
	case "list":
		return runList(args[1:], stdout, stderr)
	case "explain":
		return runExplain(args[1:], stdout, stderr)
	case "report-schema":
		if len(args) > 1 {
			fmt.Fprintf(stderr, "%s: report-schema takes no arguments\n", tool)
			return exitUsage
		}
		if _, err := stdout.Write(conformance.InfraReportSchema); err != nil {
			return exitInternal
		}
		return exitPass
	case "version":
		cat, base, err := load()
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", tool, err)
			return exitInternal
		}
		fmt.Fprintf(stdout, "%s %s (schemas %s, baseline %s, catalog %s, report %d)\n", tool, version, schema.Version, base, cat.Baseline, report.InfraVersion)
		return exitPass
	case "help", "-h", "--help", "-help":
		fmt.Fprint(stdout, usage)
		return exitPass
	}
	fmt.Fprintf(stderr, "%s: unknown command %q\n%s", tool, args[0], usage)
	return exitUsage
}

// load returns the D8 catalog and the guideline commit of the spec baseline.
func load() (*catalog.Catalog, string, error) {
	cat, err := catalog.Load(catalog.D8File)
	if err != nil {
		return nil, "", err
	}
	spec, err := topic.Load()
	if err != nil {
		return nil, "", err
	}
	return cat, spec.Baseline, nil
}

// reports is the repeatable flag --report <format>=<path>.
type reports []string

func (r *reports) String() string { return strings.Join(*r, ",") }

func (r *reports) Set(v string) error {
	f, p, ok := strings.Cut(v, "=")
	if !ok || p == "" {
		return fmt.Errorf("want <format>=<path>, got %q", v)
	}
	if !validFormat(f) {
		return fmt.Errorf("unknown report format %q (text, json, junit)", f)
	}
	*r = append(*r, v)
	return nil
}

func validFormat(f string) bool { return f == "text" || f == "json" || f == "junit" }

func runImage(args []string, stdout, stderr io.Writer) int {
	usageErr := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "%s: %s\n", tool, fmt.Sprintf(format, a...))
		return exitUsage
	}
	fl := flag.NewFlagSet("image", flag.ContinueOnError)
	fl.SetOutput(stderr)
	level := fl.Int("level", 3, "declared compliance level (1, 2, 3)")
	format := fl.String("format", "text", "output on standard output: text, json, junit")
	allowSkip := fl.Bool("allow-skip", false, "exit 0 instead of 4 if a mandatory requirement could not be judged")
	var reps reports
	fl.Var(&reps, "report", "write a report file FORMAT=PATH (repeatable)")
	if err := fl.Parse(args); err != nil {
		return exitUsage
	}
	if fl.NArg() != 1 {
		return usageErr("image takes exactly one OCI layout directory or oci-archive")
	}
	if *level < 1 || *level > 3 {
		return usageErr("--level must be 1, 2 or 3")
	}
	if !validFormat(*format) {
		return usageErr("unknown --format %q (text, json, junit)", *format)
	}
	src := fl.Arg(0)
	cat, base, err := load()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	reqs, err := verdict.LoadRequirements()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	started := time.Now().UTC()
	res, err := image.Check(src)
	if err != nil {
		if errors.Is(err, oci.ErrNotLayout) || errors.Is(err, fs.ErrNotExist) {
			return usageErr("%v", err)
		}
		fmt.Fprintf(stderr, "%s: %s: %v\n", tool, src, err)
		return exitInternal
	}
	rep := &report.Infra{Report: report.InfraVersion, Tool: tool, Version: version, Schemas: schema.Version, Baseline: base,
		Catalog: cat.Baseline, Command: append([]string{tool, "image"}, args...), Mode: "image", Input: src,
		Form: res.Image.Form, Level: *level, Started: started, Image: summarize(res.Image)}
	rep.Tests = verdict.Build(image.Tests(cat), res.Observations, reqs, *level)
	rep.Summary = verdict.Summary(rep.Tests)
	rep.Exit = verdict.Exit(rep.Tests, *allowSkip)
	rep.Ended = time.Now().UTC()
	return writeReports(rep, *format, reps, stdout, stderr)
}

func summarize(img *oci.Image) *report.Image {
	out := &report.Image{Digest: img.Digest, MediaType: img.MediaType, Index: img.Index, Algorithms: img.Algorithms,
		Problems: len(img.Problems), Manifests: []report.Manifest{}}
	for _, m := range img.Manifests {
		rm := report.Manifest{Digest: m.Descriptor.Digest, MediaType: m.MediaType, Runnable: m.Runnable, Layers: len(m.Layers)}
		if rm.MediaType == "" {
			rm.MediaType = m.Descriptor.MediaType
		}
		if m.Platform.OS != "" || m.Platform.Architecture != "" {
			rm.Platform = m.Platform.String()
		}
		if m.Runnable {
			_, rm.Supported = image.Supported(m.Platform)
		}
		out.Manifests = append(out.Manifests, rm)
	}
	return out
}

// writeReports writes the --report files and the report on standard output
// and returns the exit code of the run.
func writeReports(rep *report.Infra, format string, reps reports, stdout, stderr io.Writer) int {
	write := func(format string, w io.Writer) error {
		switch format {
		case "json":
			return rep.JSON(w)
		case "junit":
			return rep.JUnit(w)
		}
		return rep.Text(w)
	}
	for _, r := range reps {
		f, path, _ := strings.Cut(r, "=")
		var buf bytes.Buffer
		if err := write(f, &buf); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", tool, err)
			return exitInternal
		}
		if err := os.WriteFile(filepath.Clean(path), buf.Bytes(), 0o644); err != nil {
			fmt.Fprintf(stderr, "%s: report: %v\n", tool, err)
			return exitInternal
		}
	}
	if err := write(format, stdout); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	return rep.Exit
}
