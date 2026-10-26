// Package spec embeds the machine-readable data files of the OEC v2 spec
// baseline (ADR-0018) for Go consumers such as the conformance tool D7
// (ADR-0019 §5: D7 may use the data files of oec/spec, not the internals of
// its tools). It holds data only.
package spec

import "embed"

// FS holds the data files D7 validate needs: the generated topic patterns,
// the Method x Resource matrix and the requirements catalog.
//
//go:embed topic/topic-patterns.json matrix/method-resource.yaml requirements/*.yaml
var FS embed.FS

// Paths of the files in FS.
const (
	PatternsFile     = "topic/topic-patterns.json"
	MatrixFile       = "matrix/method-resource.yaml"
	RequirementsGlob = "requirements/*.yaml"
)
