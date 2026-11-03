package d7run

// The D7 tests pinned for a micro SDK device (C-093 … C-095), shared by
// the host runner (cmd/oecm-host) and the QEMU runner (cmd/oecm-qemu,
// C-096): the same core, the same simulated sensor (examples/sensor).

// Level1Pass: the 21 D7 tests that pass against the micro core in the
// host runner at Level 1 (C-093) – the config service's list (C-082)
// without T7-ADR-01: the device sends no requests. T7-ADR-03 judges the
// Identification Links of its publications. A Level 1 device answers no
// Get, Set or Call (REQ-CMP-017 applies from Level 2): D7 warns on five
// tests for the missing SHOULD answers (T7-GET-05, T7-SET-01, T7-SET-02,
// T7-SET-06, T7-SET-07) and skips the other 29 – the Level 2 and 3 tests
// (T7-MON-13, T7-SET-03), the request tests without an answer to judge
// (T7-GET-01 … 04, 06 … 09, T7-SET-04, 05, 08 … 12, T7-ACT-*), T7-ADR-01
// (no request sent), and the monitor tests whose trigger the run does not
// contain (T7-MON-07, 09, 14, 15, 17, 18, T7-ADR-02).
var Level1Pass = []string{
	"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-04", "T7-MON-05", "T7-MON-06", "T7-MON-08", "T7-MON-10", "T7-MON-11", "T7-MON-12", "T7-MON-16", "T7-MON-19",
	"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04",
	"T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	"T7-ADR-03",
}

// Level2Pass: the 38 D7 tests that pass against the micro core in the
// host runner at Level 2 (C-094), the simulated temperature sensor of
// examples/sensor. D7 warns on three SHOULDs of features of Level 3 of the micro
// SDK (C-095): ReferenceDesignation (T7-GET-03, T7-GET-04) and
// NewDataSetWriterId (T7-ACT-01). It skips 14: T7-GET-07, T7-SET-04,
// T7-SET-09 and T7-SET-10 need ReferenceDesignation; T7-ACT-02 … 04 need
// NewDataSetWriterId; T7-SET-12 needs a Syslog publication (the micro SDK
// has none, ADR-0030); T7-MON-07, T7-ADR-01 and T7-ADR-02 judge requests
// of the CUT (none sent); T7-MON-14 and T7-MON-17 observed nothing to
// judge; T7-SET-03 is Level 3.
var Level2Pass = []string{
	"T7-MON-01", "T7-MON-02", "T7-MON-03", "T7-MON-04", "T7-MON-05", "T7-MON-06", "T7-MON-08", "T7-MON-09", "T7-MON-10", "T7-MON-11", "T7-MON-12", "T7-MON-13", "T7-MON-15", "T7-MON-16", "T7-MON-18", "T7-MON-19",
	"T7-CON-01", "T7-CON-02", "T7-CON-03", "T7-CON-04",
	"T7-LIFE-01", "T7-LIFE-02", "T7-LIFE-03", "T7-LIFE-04",
	"T7-ADR-03",
	"T7-GET-01", "T7-GET-02", "T7-GET-05", "T7-GET-06", "T7-GET-08", "T7-GET-09",
	"T7-SET-01", "T7-SET-02", "T7-SET-05", "T7-SET-06", "T7-SET-07", "T7-SET-08", "T7-SET-11",
}

// Level3Pass: the 50 D7 tests that pass against the micro core in the
// host runner at Level 3 (C-095): the Level 2 list, T7-SET-03, the
// ReferenceDesignation tests (T7-GET-03, T7-GET-04, T7-GET-07, T7-SET-04,
// T7-SET-09, T7-SET-10; the run starts with a stored ReferenceDesignation,
// RefdesSeed), T7-MON-14 (its publication on change) and the
// NewDataSetWriterId tests (T7-ACT-01 … 04). D7 skips five: T7-SET-12
// needs a Syslog publication (none in the micro SDK); T7-MON-07, T7-ADR-01
// and T7-ADR-02 judge requests of the CUT (none sent); T7-MON-17 observed
// no Set Config of a group with mandatory objects.
var Level3Pass = append(append([]string{}, Level2Pass...),
	"T7-MON-14", "T7-GET-03", "T7-GET-04", "T7-GET-07", "T7-SET-03", "T7-SET-04", "T7-SET-09", "T7-SET-10",
	"T7-ACT-01", "T7-ACT-02", "T7-ACT-03", "T7-ACT-04")

// RefdesSeed is a stored ReferenceDesignation in the form the core stores
// (canonical member order, oecm_l3.c), as after a Set at commissioning.
const RefdesSeed = `{"Function":{"Value":"=OEC.MICRO","Parent":{"Value":"=OEC","IdentificationLink":"oi4.org/OEC/OEC/1"}},"Location":{"Value":"+HOST"}}`
