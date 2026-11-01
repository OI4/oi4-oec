// Command oec-buildlib builds the static C ABI library of the Rust SDK
// (crate oec-ffi) and puts it where package oec links it:
// lib/<GOOS>_<GOARCH>/liboec_ffi.a (ADR-0027 §2). Run it in oec/sdk/go:
//
//	go run ./cmd/oec-buildlib                      # the host
//	go run ./cmd/oec-buildlib -target aarch64-unknown-linux-gnu
//
// A cross target needs the Rust target (rustup target add) and a C cross
// toolchain for its C parts (ring/aws-lc-rs); see README.md.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	target := flag.String("target", "", "Rust target triple (default: the host)")
	profile := flag.String("profile", "release", "Cargo profile: release or debug")
	dir := flag.String("dir", ".", "the directory of the Go module oec/sdk/go")
	flag.Parse()
	if err := build(*dir, *target, *profile); err != nil {
		fmt.Fprintln(os.Stderr, "oec-buildlib:", err)
		os.Exit(1)
	}
}

func build(dir, target, profile string) error {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	if target != "" {
		var err error
		if goos, goarch, err = goTarget(target); err != nil {
			return err
		}
	}
	if profile != "release" && profile != "debug" {
		return fmt.Errorf("profile %q: release or debug", profile)
	}
	manifest, err := filepath.Abs(filepath.Join(dir, "..", "rust", "Cargo.toml"))
	if err != nil {
		return err
	}
	args := []string{"build", "-p", "oec-ffi", "--manifest-path", manifest}
	if profile == "release" {
		args = append(args, "--release")
	}
	if target != "" {
		args = append(args, "--target", target)
	}
	cargo := exec.Command("cargo", args...)
	cargo.Stdout, cargo.Stderr = os.Stderr, os.Stderr
	if err := cargo.Run(); err != nil {
		return fmt.Errorf("cargo %s: %w", strings.Join(args, " "), err)
	}
	targetDir, err := cargoTargetDir(manifest)
	if err != nil {
		return err
	}
	src := filepath.Join(targetDir, target, profile, "liboec_ffi.a")
	dst := filepath.Join(dir, "lib", goos+"_"+goarch, "liboec_ffi.a")
	if err := copyFile(src, dst); err != nil {
		return err
	}
	fmt.Println(dst)
	return nil
}

// goTarget maps a Rust target triple to GOOS and GOARCH.
func goTarget(triple string) (string, string, error) {
	parts := strings.Split(triple, "-")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("target %q is no Rust target triple", triple)
	}
	var goos string
	switch {
	case strings.Contains(triple, "-linux-"):
		goos = "linux"
	case strings.Contains(triple, "-apple-darwin"):
		goos = "darwin"
	case strings.Contains(triple, "-windows-"):
		goos = "windows"
	default:
		return "", "", fmt.Errorf("target %q: unsupported OS", triple)
	}
	arch := map[string]string{"x86_64": "amd64", "aarch64": "arm64", "armv7": "arm", "i686": "386"}[parts[0]]
	if arch == "" {
		return "", "", fmt.Errorf("target %q: unsupported architecture", triple)
	}
	return goos, arch, nil
}

// cargoTargetDir is the target directory of the workspace (honors
// CARGO_TARGET_DIR and the Cargo configuration).
func cargoTargetDir(manifest string) (string, error) {
	var out bytes.Buffer
	cmd := exec.Command("cargo", "metadata", "--format-version", "1", "--no-deps",
		"--manifest-path", manifest)
	cmd.Stdout, cmd.Stderr = &out, os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("cargo metadata: %w", err)
	}
	var meta struct {
		TargetDirectory string `json:"target_directory"`
	}
	if err := json.Unmarshal(out.Bytes(), &meta); err != nil {
		return "", fmt.Errorf("cargo metadata: %w", err)
	}
	if meta.TargetDirectory == "" {
		return "", errors.New("cargo metadata: no target_directory")
	}
	return meta.TargetDirectory, nil
}

// copyFile copies src to dst through a temporary file in dst's directory.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".liboec_ffi-*.a")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dst)
}
