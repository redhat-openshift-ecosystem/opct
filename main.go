package main

import (
	"embed"

	cmd "github.com/redhat-openshift-ecosystem/opct/cmd/opct"
	"github.com/redhat-openshift-ecosystem/opct/internal/assets"
)

//go:embed data/templates
var vfs embed.FS

func main() {
	assets.UpdateData(&vfs)
	cmd.Execute()
}
