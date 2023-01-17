//go:build flow
// +build flow

package main

import _ "embed"

//go:embed node.go.flow.template
var nodeTmpl []byte
