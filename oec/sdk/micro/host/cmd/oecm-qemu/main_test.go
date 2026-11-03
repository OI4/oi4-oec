package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/OI4/oec/sdk/micro/host/internal/d7run"
)

// TestMain runs the test binary as the runner when the D7 harness starts
// it (OECM_QEMU_MAIN=1), as in cmd/oecm-host.
func TestMain(m *testing.M) {
	if os.Getenv("OECM_QEMU_MAIN") == "1" {
		os.Exit(run(os.Args[1:], os.Getenv("OEC_ROOT")))
	}
	os.Exit(m.Run())
}

const testCA = "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----\n"

func TestProvisioning(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		d7run.Write(t, filepath.Join(root, name), content)
	}
	write("etc/hostname", "dev-1\n")
	write("etc/oi4/mqtt/broker.json", `{"Address": "localhost", "SecurePort": 8883, "MaxPacketSize": 1000}`)
	if _, err := loadProvisioning(root, "10.0.2.2"); err == nil || !strings.Contains(err.Error(), "CA") {
		t.Fatalf("no CA: %v", err)
	}
	write("etc/oi4/certs/ca.pem", testCA)
	if _, err := loadProvisioning(root, "10.0.2.2"); err == nil || !strings.Contains(err.Error(), "authentication") {
		t.Fatalf("no credentials: %v", err)
	}
	write("run/secrets/oi4/credentials", "dXNlcjpwYTpzcw==\n") // "user:pa:ss"
	p, err := loadProvisioning(root, "10.0.2.2")
	if err != nil {
		t.Fatal(err)
	}
	if p.serial != "dev-1" || p.uri != "mqtts://10.0.2.2:8883" || p.maxPacket != 1000 ||
		p.user != "user" || p.password != "pa:ss" || p.auth() != "credentials" {
		t.Fatalf("provisioning %+v", p)
	}
	for addr, uri := range map[string]string{
		"127.0.0.1": "mqtts://10.0.2.2:8883", "::1": "mqtts://10.0.2.2:8883",
		"broker.local": "mqtts://broker.local:8883", "fd00::1": "mqtts://[fd00::1]:8883",
	} {
		write("etc/oi4/mqtt/broker.json", `{"Address": "`+addr+`", "SecurePort": 8883, "MaxPacketSize": 1000}`)
		if p, err = loadProvisioning(root, "10.0.2.2"); err != nil || p.uri != uri {
			t.Errorf("Address %s: %v %+v", addr, err, p)
		}
	}
	p.level = 3
	dir := t.TempDir()
	csv, err := p.csv(dir)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(csv)
	want := "key,type,encoding,value\noec,namespace,,\n" +
		"uri,file,string," + filepath.Join(dir, "uri.txt") + "\n" +
		"ca,file,string," + filepath.Join(dir, "ca.txt") + "\n" +
		"serial,file,string," + filepath.Join(dir, "serial.txt") + "\n" +
		"user,file,string," + filepath.Join(dir, "user.txt") + "\n" +
		"password,file,string," + filepath.Join(dir, "password.txt") + "\n" +
		"level,data,u8,3\nmax_packet,data,u32,1000\n"
	if string(raw) != want {
		t.Fatalf("csv\n%s\nwant\n%s", raw, want)
	}
	if pw, _ := os.ReadFile(filepath.Join(dir, "password.txt")); string(pw) != "pa:ss" {
		t.Fatalf("password file %q", pw)
	}

	write("run/secrets/oi4/private_key.pem", "KEY")
	write("etc/oi4/certs/dev-1.pem", "CERT")
	if p, err = loadProvisioning(root, "10.0.2.2"); err != nil || p.auth() != "client certificate" ||
		string(p.cert) != "CERT" || string(p.key) != "KEY" {
		t.Fatalf("client certificate wins: %v %+v", err, p)
	}
	write("run/secrets/oi4/passphrase", "x")
	if _, err := loadProvisioning(root, "10.0.2.2"); err == nil || !strings.Contains(err.Error(), "encrypted") {
		t.Fatalf("passphrase: %v", err)
	}
}

func TestPartitions(t *testing.T) {
	img := make([]byte, flashSize)
	for i := partTableOffset; i < partTableOffset+partTableSize; i++ {
		img[i] = 0xFF
	}
	entry := func(i int, label string, offset, size uint32) {
		e := img[partTableOffset+32*i:]
		binary.LittleEndian.PutUint16(e, 0x50AA)
		binary.LittleEndian.PutUint32(e[4:], offset)
		binary.LittleEndian.PutUint32(e[8:], size)
		copy(e[12:28], make([]byte, 16))
		copy(e[12:28], label)
	}
	if _, err := partitions(img); err == nil {
		t.Fatal("empty table accepted")
	}
	entry(0, "nvs", 0x9000, 0x6000)
	entry(1, "oecprov", 0x10000, 0x6000)
	binary.LittleEndian.PutUint16(img[partTableOffset+64:], 0xEBEB) // MD5 entry
	ps, err := partitions(img)
	if err != nil || len(ps) != 2 {
		t.Fatalf("%v %+v", err, ps)
	}
	if p, err := find(ps, "oecprov"); err != nil || p.offset != 0x10000 || p.size != 0x6000 {
		t.Fatalf("oecprov %v %+v", err, p)
	}
	if _, err := find(ps, "factory"); err == nil {
		t.Fatal("missing partition found")
	}
	entry(1, "oecprov", flashSize-0x1000, 0x6000)
	if _, err := partitions(img); err == nil || !strings.Contains(err.Error(), "beyond") {
		t.Fatalf("partition beyond the image: %v", err)
	}
	binary.LittleEndian.PutUint16(img[partTableOffset+32:], 0x1234)
	if _, err := partitions(img); err == nil || !strings.Contains(err.Error(), "magic") {
		t.Fatalf("bad magic: %v", err)
	}
}

// qemuD7 runs D7 against the example firmware in the Espressif QEMU
// (opt-in: OECM_QEMU_BUILD = the build of the variant qemu, OECM_QEMU =
// the Espressif qemu-system-xtensa, the Python of ESP-IDF on the PATH or
// OECM_QEMU_PYTHON). Without them the test is skipped, also with
// OECM_REQUIRE=1: QEMU is not part of the gate (ADR-0030 §8). The device
// passes the same pinned tests as the core in the host runner.
func qemuD7(t *testing.T, level int, pass []string, files map[string]string) {
	build, qemu := os.Getenv("OECM_QEMU_BUILD"), os.Getenv("OECM_QEMU")
	if build == "" || qemu == "" {
		t.Skip("OECM_QEMU_BUILD and OECM_QEMU not set (opt-in, examples/esp-idf/build.sh qemu)")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	d7run.Run(t, d7run.D7{
		Exe:         exe,
		Args:        []string{"-build", build, "-qemu", qemu, "-level", strconv.Itoa(level)},
		Env:         []string{"OECM_QEMU_MAIN=1"},
		Hostname:    "dev-1",
		App:         "OTConnector/oi4.org/OEC,20Micro,20ESP32/OECM-ESP32/dev-1",
		Level:       level,
		Pass:        pass,
		Report:      "OECM_D7_REPORT",
		Files:       files,
		ProxyNames:  []string{"10.0.2.2"},
		Timeout:     5 * time.Second,
		RestartWait: time.Minute,
	})
}

func TestQEMUD7Level1(t *testing.T) { qemuD7(t, 1, d7run.Level1Pass, nil) }

func TestQEMUD7Level2(t *testing.T) { qemuD7(t, 2, d7run.Level2Pass, nil) }

func TestQEMUD7Level3(t *testing.T) {
	qemuD7(t, 3, d7run.Level3Pass, map[string]string{"var/lib/oecm-qemu/oecm.refdes": d7run.RefdesSeed})
}
