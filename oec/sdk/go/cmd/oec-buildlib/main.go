// Command oec-buildlib builds the static C ABI library of the Rust SDK
// (crate oec-ffi) and puts it where package oec links it:
// lib/<GOOS>_<GOARCH>/liboec_ffi.a (ADR-0027 §2). Run it in oec/sdk/go:
//
//	go run ./cmd/oec-buildlib                      # the host
//	go run ./cmd/oec-buildlib -target aarch64-unknown-linux-gnu
//
// A cross target needs the Rust target (rustup target add) and a C cross
// toolchain for its C parts (ring/aws-lc-rs); see README.md.
//
// The LLVM bitcode that the precompiled Rust standard library embeds in its
// objects is removed with llvm-objcopy of the rustup component llvm-tools:
// since go1.26.11/go1.27.4 cgo rejects a library in a -L directory of the
// module that contains LTO sections (C-114).
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
	objcopy, err := llvmObjcopy()
	if err != nil {
		return err
	}
	if err := copyStripped(src, dst, objcopy); err != nil {
		return err
	}
	fmt.Println(dst)
	return nil
}

// bitcodeSections are the sections of embedded LLVM bitcode (Mach-O and
// ELF/COFF names); llvm-objcopy ignores the names of the other format.
var bitcodeSections = []string{"__LLVM,__bitcode", "__LLVM,__cmdline", ".llvmbc", ".llvmcmd"}

// llvmObjcopy is the llvm-objcopy of the rustup component llvm-tools of the
// rustc in PATH (it handles every target), else llvm-objcopy in PATH.
func llvmObjcopy() (string, error) {
	exe := "llvm-objcopy"
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	sysroot, err1 := exec.Command("rustc", "--print", "sysroot").Output()
	vv, err2 := exec.Command("rustc", "-vV").Output()
	if err1 == nil && err2 == nil {
		for _, line := range strings.Split(string(vv), "\n") {
			if host, ok := strings.CutPrefix(strings.TrimSpace(line), "host: "); ok {
				p := filepath.Join(strings.TrimSpace(string(sysroot)), "lib", "rustlib", host, "bin", exe)
				if _, err := os.Stat(p); err == nil {
					return p, nil
				}
			}
		}
	}
	if p, err := exec.LookPath("llvm-objcopy"); err == nil {
		return p, nil
	}
	return "", errors.New("llvm-objcopy not found: rustup component add llvm-tools")
}

// copyStripped copies src to dst through a temporary file in dst's
// directory and removes the bitcode sections from the copy.
func copyStripped(src, dst, objcopy string) error {
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
	args := make([]string, 0, len(bitcodeSections)+1)
	for _, s := range bitcodeSections {
		args = append(args, "--remove-section="+s)
	}
	strip := exec.Command(objcopy, append(args, tmp.Name())...)
	strip.Stdout, strip.Stderr = os.Stderr, os.Stderr
	if err := strip.Run(); err != nil {
		return fmt.Errorf("%s: %w", objcopy, err)
	}
	return os.Rename(tmp.Name(), dst)
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
