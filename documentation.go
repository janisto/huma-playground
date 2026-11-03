// Package playground embeds the two public API usage guides from this build.
package playground

import _ "embed"

//go:embed llms.txt
var UsageIndex []byte

//go:embed llms-full.txt
var UsageGuide []byte
