package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The partition table of an ESP32 image: at 0x8000, entries of 32 octets
// (magic 0x50AA little endian, type, subtype, offset, size, label of 16
// octets, flags), ended by an MD5 entry (0xEBEB) or erased flash.
const (
	partTableOffset = 0x8000
	partTableSize   = 0xC00
	flashSize       = 4 << 20
)

type partition struct {
	label  string
	offset uint32
	size   uint32
}

func partitions(img []byte) ([]partition, error) {
	if len(img) < partTableOffset+partTableSize {
		return nil, errors.New("flash image too small for a partition table")
	}
	var ps []partition
	for off := partTableOffset; off+32 <= partTableOffset+partTableSize; off += 32 {
		e := img[off : off+32]
		magic := binary.LittleEndian.Uint16(e)
		if magic != 0x50AA {
			if magic == 0xEBEB || magic == 0xFFFF {
				break
			}
			return nil, fmt.Errorf("partition table: entry at 0x%x has magic 0x%04x", off, magic)
		}
		p := partition{
			label:  string(bytes.TrimRight(e[12:28], "\x00")),
			offset: binary.LittleEndian.Uint32(e[4:]),
			size:   binary.LittleEndian.Uint32(e[8:]),
		}
		if uint64(p.offset)+uint64(p.size) > uint64(len(img)) {
			return nil, fmt.Errorf("partition table: %s ends beyond the image", p.label)
		}
		ps = append(ps, p)
	}
	if len(ps) == 0 {
		return nil, errors.New("partition table: no entry")
	}
	return ps, nil
}

func find(ps []partition, label string) (partition, error) {
	for _, p := range ps {
		if p.label == label {
			return p, nil
		}
	}
	return partition{}, fmt.Errorf("partition table: no partition %q", label)
}

// tools are the Python modules of ESP-IDF the runner calls (the Python
// environment of export.sh): esptool and the NVS partition generator.
type tools struct {
	python string
}

func (t tools) run(dir string, args ...string) error {
	cmd := exec.Command(t.python, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v\n%s", t.python, strings.Join(args, " "), err, out)
	}
	return nil
}

// nvs generates an NVS partition image of size octets from a CSV.
func (t tools) nvs(csv string, size uint32) ([]byte, error) {
	dir, err := os.MkdirTemp("", "oecm-qemu-nvs")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "nvs.bin")
	if err := t.run(dir, "-m", "esp_idf_nvs_partition_gen", "generate", "--outdir", dir, csv, out,
		fmt.Sprintf("0x%x", size)); err != nil {
		return nil, err
	}
	img, err := os.ReadFile(out)
	if err == nil && uint32(len(img)) != size {
		err = fmt.Errorf("NVS image of %d octets, partition %d", len(img), size)
	}
	return img, err
}

// createFlash merges the build (flash_args of idf.py) into a 4 MB image
// and seeds the partition "nvs" with the records of the core found in
// state (oecm.refdes, oecm.wid: files as the host runner keeps them), as
// after a commissioning.
func (t tools) createFlash(build, flash, state string) error {
	if err := t.run(build, "-m", "esptool", "--chip", "esp32", "merge-bin", "--pad-to-size", "4MB",
		"-o", flash, "@flash_args"); err != nil {
		return err
	}
	var seeds []string
	for _, key := range []string{"oecm.refdes", "oecm.wid"} {
		if exists(filepath.Join(state, key)) {
			seeds = append(seeds, key)
		}
	}
	if len(seeds) == 0 {
		return nil
	}
	img, err := os.ReadFile(flash)
	if err != nil {
		return err
	}
	ps, err := partitions(img)
	if err != nil {
		return err
	}
	nvsPart, err := find(ps, "nvs")
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("key,type,encoding,value\noecm,namespace,,\n")
	for _, key := range seeds {
		fmt.Fprintf(&b, "%s,file,binary,%s\n", key, filepath.Join(state, key))
	}
	csv := filepath.Join(state, "nvs-seed.csv")
	if err := os.WriteFile(csv, []byte(b.String()), 0o600); err != nil {
		return err
	}
	part, err := t.nvs(csv, nvsPart.size)
	if err != nil {
		return err
	}
	copy(img[nvsPart.offset:], part)
	return os.WriteFile(flash, img, 0o600)
}

// provision writes the partition "oecprov" of the flash image anew.
func (t tools) provision(flash string, p *provisioning) error {
	img, err := os.ReadFile(flash)
	if err != nil {
		return err
	}
	if len(img) != flashSize {
		return fmt.Errorf("%s: %d octets, want %d", flash, len(img), flashSize)
	}
	ps, err := partitions(img)
	if err != nil {
		return err
	}
	prov, err := find(ps, "oecprov")
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "oecm-qemu-prov")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	csv, err := p.csv(dir)
	if err != nil {
		return err
	}
	part, err := t.nvs(csv, prov.size)
	if err != nil {
		return err
	}
	copy(img[prov.offset:], part)
	return os.WriteFile(flash, img, 0o600)
}
