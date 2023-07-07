package main

import (
	"embed"

	cmd "github.com/redhat-openshift-ecosystem/provider-certification-tool/cmd/opct"
	"github.com/redhat-openshift-ecosystem/provider-certification-tool/internal/assets"
)

//go:embed data/templates
var vfs embed.FS

func main() {
	assets.UpdateData(&vfs)
	cmd.Execute()
}
