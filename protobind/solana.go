//go:build solana
// +build solana

package main

import _ "embed"

//go:embed node.go.solana.template
var nodeTmpl []byte
