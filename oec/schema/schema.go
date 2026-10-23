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
