package oec

// The link flags of the static C ABI library (ADR-0027 §2). By default the
// package links lib/<GOOS>_<GOARCH>/liboec_ffi.a (built by
// `go run ./cmd/oec-buildlib`) and includes the committed header of
// oec-ffi. With the build tag oec_external the defaults are left out and
// CGO_CFLAGS/CGO_LDFLAGS name the header directory, the library and the
// system libraries (`rustc --print native-static-libs`). On darwin the
// list leaves out -lSystem: the Go linker adds it (no duplicate warning).

/*
#cgo !oec_external CFLAGS: -I${SRCDIR}/../rust/oec-ffi/include
#cgo !oec_external,darwin,arm64 LDFLAGS: -L${SRCDIR}/lib/darwin_arm64 -loec_ffi
#cgo !oec_external,darwin,amd64 LDFLAGS: -L${SRCDIR}/lib/darwin_amd64 -loec_ffi
#cgo !oec_external,linux,amd64 LDFLAGS: -L${SRCDIR}/lib/linux_amd64 -loec_ffi
#cgo !oec_external,linux,arm64 LDFLAGS: -L${SRCDIR}/lib/linux_arm64 -loec_ffi
#cgo !oec_external,linux,arm LDFLAGS: -L${SRCDIR}/lib/linux_arm -loec_ffi
#cgo !oec_external,darwin LDFLAGS: -liconv -lc -lm
#cgo !oec_external,linux LDFLAGS: -lgcc_s -lutil -lrt -lpthread -lm -ldl -lc
*/
import "C"
