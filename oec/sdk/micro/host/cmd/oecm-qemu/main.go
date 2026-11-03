// Command oecm-qemu runs the ESP-IDF example of the micro SDK
// (examples/esp-idf, build variant qemu: an ESP32 with OpenCores Ethernet)
// in the Espressif QEMU as a smart device (ADR-0030 §8, C-096). Like
// oecm-host it reads the broker configuration from the layout of the
// container storage below OEC_ROOT and writes it into the provisioning
// partition "oecprov" of the flash image on every start; a loopback
// broker address becomes the host address of the QEMU user network
// (10.0.2.2). The flash image (and with it the NVS records of Level 3)
// lives in the state directory (default OEC_ROOT/var/lib/oecm-qemu) across
// restarts; on its creation the records oecm.refdes and oecm.wid found there
// as files are seeded into the partition "nvs".
//
//	oecm-qemu -build DIR -qemu PATH [-level 1|2|3] [-state DIR] [-python P]
//
// The firmware's serial console is the runner's stdin/stdout: the runner
// sets the clock ("time <unix ms>") when the firmware waits for it (no
// SNTP server needed) and copies the console to stderr. SIGTERM or SIGINT
// sends "stop" (the firmware publishes the Close and disconnects), waits
// for "oecm: stopped" and ends QEMU (exit 0).
//
// Tools: the Espressif QEMU (qemu-system-xtensa with the machine esp32,
// -qemu or OECM_QEMU) and the Python of ESP-IDF with esptool and
// esp_idf_nvs_partition_gen (-python or OECM_QEMU_PYTHON, default python).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv("OEC_ROOT")))
}

func run(args []string, root string) int {
	logger := log.New(os.Stderr, "oecm-qemu: ", log.LstdFlags|log.Lmicroseconds)
	fs := flag.NewFlagSet("oecm-qemu", flag.ContinueOnError)
	build := fs.String("build", os.Getenv("OECM_QEMU_BUILD"), "build directory of the variant qemu (examples/esp-idf/build.sh qemu); default OECM_QEMU_BUILD")
	qemu := fs.String("qemu", os.Getenv("OECM_QEMU"), "the Espressif qemu-system-xtensa; default OECM_QEMU")
	python := fs.String("python", envOr("OECM_QEMU_PYTHON", "python"), "Python of ESP-IDF (esptool, esp_idf_nvs_partition_gen); default OECM_QEMU_PYTHON or python")
	level := fs.Int("level", 3, "Messaging level of the device (1, 2 or 3)")
	state := fs.String("state", "", "directory of the flash image; default: OEC_ROOT/var/lib/oecm-qemu")
	hostAddr := fs.String("host-addr", "10.0.2.2", "address of the host in the QEMU user network (for a loopback broker address)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *level < 1 || *level > 3 || *build == "" || *qemu == "" {
		logger.Print("-level 1 … 3, -build and -qemu are required")
		return 2
	}
	if root == "" {
		root = "/"
	}
	if *state == "" {
		*state = filepath.Join(root, "var/lib/oecm-qemu")
	}
	p, err := loadProvisioning(root, *hostAddr)
	if err != nil {
		logger.Print(err)
		return 1
	}
	p.level = *level
	if err := os.MkdirAll(*state, 0o755); err != nil {
		logger.Print(err)
		return 1
	}
	t := tools{python: *python}
	flash := filepath.Join(*state, "flash.bin")
	if !exists(flash) {
		if err := t.createFlash(*build, flash, *state); err != nil {
			logger.Print(err)
			return 1
		}
	}
	if err := t.provision(flash, p); err != nil {
		logger.Print(err)
		return 1
	}
	logger.Printf("serial %s, broker %s, %s, Level %d", p.serial, p.uri, p.auth(), p.level)
	return emulate(logger, *qemu, flash)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// emulate runs QEMU until a signal or its end.
func emulate(logger *log.Logger, qemu, flash string) int {
	cmd := exec.Command(qemu, "-machine", "esp32", "-display", "none", "-monitor", "none", "-serial", "stdio",
		"-drive", "file="+flash+",if=mtd,format=raw", "-nic", "user,model=open_eth")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.Print(err)
		return 1
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logger.Print(err)
		return 1
	}
	cmd.Stderr = os.Stderr
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	if err := cmd.Start(); err != nil {
		logger.Print(err)
		return 1
	}
	stopped := make(chan struct{})
	ended := make(chan struct{})
	go console(stdout, stdin, stopped)
	go func() {
		_ = cmd.Wait()
		close(ended)
	}()
	select {
	case <-ended:
		logger.Print("QEMU ended")
		return 1
	case sig := <-stop:
		_, _ = io.WriteString(stdin, "stop\n")
		code := 0
		select {
		case <-stopped:
		case <-ended:
			code = 1
		case <-time.After(10 * time.Second):
			logger.Printf("%v: no stop of the firmware within 10 s", sig)
			code = 1
		}
		_ = cmd.Process.Kill()
		<-ended
		logger.Printf("%v: QEMU ended", sig)
		return code
	}
}

// console copies the serial output to stderr, answers the firmware's wait
// for the clock and reports its stop.
func console(out io.Reader, in io.Writer, stopped chan<- struct{}) {
	r := bufio.NewReader(out)
	once := false
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			fmt.Fprint(os.Stderr, line)
			if strings.Contains(line, "waiting for the time") {
				fmt.Fprintf(in, "time %d\n", time.Now().UnixMilli())
			}
			if strings.Contains(line, "oecm: stopped") && !once {
				once = true
				close(stopped)
			}
		}
		if err != nil {
			return
		}
	}
}
