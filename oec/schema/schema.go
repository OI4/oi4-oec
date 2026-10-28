// Package schema embeds the JSON Schema 2020-12 files of D1 (ADR-0019) for
// Go consumers such as the conformance tool D7. It holds data only.
package schema

import "embed"

// FS holds the schema files below v2/ (strict flavor, the source).
//
//go:embed v2
var FS embed.FS

// BaseURI is the $id prefix of every schema file in FS.
const BaseURI = "https://schemas.openindustry4.com/oec/v2/"

// TolerantBaseURI is the $id prefix of the generated tolerant flavor.
const TolerantBaseURI = BaseURI + "tolerant/"

// Version is the version of the schema bundle, 2.MINOR.PATCH (ADR-0019 §6).
// The $ids carry the major version only; tools that embed the bundle report
// Version and Baseline with every result. A released version is frozen in
// release/<Version>.json; work after a release uses a pre-release version
// such as 2.1.0-dev (ADR-0019 amendment C-041).
const Version = "2.0.1-dev"

// Baseline is the guideline commit the bundle is derived from (the baseline
// of the spec catalog, ADR-0018).
const Baseline = "eb7889c"
