// Command oec-check is the application and messaging compliance CLI of OEC
// v2.0 (deliverable D7). The command validate checks messages offline
// against the embedded baseline (ADR-0019 §5).
//
// Exit codes: 0 every message is valid (warnings allowed), 1 at least one
// message violates a requirement, 2 usage or input error, 3 internal error.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OI4/oec/conformance/internal/report"
	"github.com/OI4/oec/conformance/internal/validate"
)

const tool = "oec-check"

// version is set by the release build (-ldflags "-X main.version=…").
var version = "dev"

const (
	exitValid    = 0
	exitInvalid  = 1
	exitUsage    = 2
	exitInternal = 3
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

const usage = `usage:
  oec-check validate --topic TOPIC [flags] [PAYLOAD.json | -]
  oec-check validate --object DataSetMetaDataType [flags] [OBJECT.json | -]
  oec-check validate [flags] MESSAGE.json | MESSAGES.jsonl | DIR ...
  oec-check version

A message file is a JSON object {"topic": "...", "payload": {...}} with the
optional keys "role" (birth, close, will), "object" and "tolerant"; a file may
hold several messages (JSONL or concatenated objects); a directory is searched for *.json and
*.jsonl files. Without a file, the payload or the messages are read from
standard input.

Exit codes: 0 valid, 1 invalid, 2 usage or input error, 3 internal error.
`

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "validate":
		return runValidate(args[1:], stdin, stdout, stderr)
	case "version":
		v, err := validate.New()
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", tool, err)
			return exitInternal
		}
		fmt.Fprintf(stdout, "%s %s (baseline %s, %d schema files)\n", tool, version, v.Baseline(), v.Schemas.Count())
		return exitValid
	case "help", "-h", "--help", "-help":
		fmt.Fprint(stdout, usage)
		return exitValid
	}
	fmt.Fprintf(stderr, "%s: unknown command %q\n%s", tool, args[0], usage)
	return exitUsage
}

type source struct {
	name string
	msg  validate.Message
}

func runValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fl := flag.NewFlagSet("validate", flag.ContinueOnError)
	fl.SetOutput(stderr)
	fl.Usage = func() { fmt.Fprint(stderr, usage) }
	topic := fl.String("topic", "", "MQTT topic of the payload")
	role := fl.String("role", "", "declared message role: birth, close or will (R-6)")
	object := fl.String("object", "", "validate a bare object without topic: "+validate.ObjectMetaData)
	tolerant := fl.Bool("tolerant", false, "tolerant receiver flavor: unknown keys are accepted")
	format := fl.String("format", "text", "report format: text or json")
	if err := fl.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitValid
		}
		return exitUsage
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "%s: unknown format %q (text, json)\n", tool, *format)
		return exitUsage
	}
	v, err := validate.New()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	srcs, err := sources(fl.Args(), stdin, validate.Message{Topic: *topic, Role: *role, Object: *object, Tolerant: *tolerant})
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitUsage
	}
	rep := report.New(tool, version, v.Baseline())
	for _, s := range srcs {
		res, err := v.Validate(s.msg)
		if errors.Is(err, validate.ErrUsage) {
			fmt.Fprintf(stderr, "%s: %s: %v\n", tool, s.name, err)
			return exitUsage
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %s: internal error: %v\n", tool, s.name, err)
			return exitInternal
		}
		rep.Add(s.name, res)
	}
	if *format == "json" {
		err = rep.JSON(stdout)
	} else {
		err = rep.Text(stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", tool, err)
		return exitInternal
	}
	if !rep.Valid {
		return exitInvalid
	}
	return exitValid
}

// sources reads the messages of the arguments. With a topic or an object
// from the flags, one argument (or standard input) is the payload;
// otherwise the arguments are message files, JSONL files or directories.
func sources(args []string, stdin io.Reader, flags validate.Message) ([]source, error) {
	read := func(name string) ([]byte, error) {
		if name == "-" {
			return io.ReadAll(stdin)
		}
		return os.ReadFile(name)
	}
	if flags.Topic != "" || flags.Object != "" {
		if len(args) > 1 {
			return nil, fmt.Errorf("--topic/--object take one payload, got %d files", len(args))
		}
		name := "-"
		if len(args) == 1 {
			name = args[0]
		}
		data, err := read(name)
		if err != nil {
			return nil, err
		}
		m := flags
		m.Payload = json.RawMessage(data)
		return []source{{name: name, msg: m}}, nil
	}
	if flags.Role != "" {
		return nil, errors.New("--role needs --topic; a message file names its role itself")
	}
	if len(args) == 0 {
		args = []string{"-"}
	}
	var out []source
	for _, a := range args {
		files := []string{a}
		if a != "-" {
			st, err := os.Stat(a)
			if err != nil {
				return nil, err
			}
			if st.IsDir() {
				if files, err = walk(a); err != nil {
					return nil, err
				}
			}
		}
		for _, f := range files {
			data, err := read(f)
			if err != nil {
				return nil, err
			}
			ms, err := messages(f, data, flags.Tolerant)
			if err != nil {
				return nil, err
			}
			out = append(out, ms...)
		}
	}
	return out, nil
}

func walk(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && (strings.HasSuffix(p, ".json") || strings.HasSuffix(p, ".jsonl")) {
			files = append(files, p)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

// messages decodes a message file: a stream of message objects – one
// object, JSONL or concatenated objects.
func messages(name string, data []byte, tolerant bool) ([]source, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var out []source
	for n := 1; ; n++ {
		var m validate.Message
		err := dec.Decode(&m)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%s: message %d: no message object: %w", name, n, err)
		}
		m.Tolerant = m.Tolerant || tolerant
		out = append(out, source{name: fmt.Sprintf("%s#%d", name, n), msg: m})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no message", name)
	}
	if len(out) == 1 {
		out[0].name = name
	}
	return out, nil
}
