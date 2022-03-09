package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"github.com/alecthomas/protobuf/compiler"
	"google.golang.org/protobuf/proto"
)

var (
	// version vars set by goreleaser
	version = "tip"
	commit  = "HEAD"
	date    = "now"

	description = `
protobuf creates FileDescriptorSet files (.pb) from Proto source files (.proto).
`
	cli struct {
		CompileConfig
		Version kong.VersionFlag `help:"Show version."`
	}
)

type CompileConfig struct {
	ProtoPath        []string `short:"I" help:"Search paths for proto imports."`
	DescriptorSetOut string   `short:"o" required:"" help:"FileDescriptorSet output file"`
	IncludeImports   bool     `help:"Include all dependencies of the input files so that the set is self-contained."`
	Files            []string `arg:"" help:"Import proto files"`
}

func main() {
	kctx := kong.Parse(&cli,
		kong.Description(description),
		kong.Vars{"version": fmt.Sprintf("%s (%s on %s)", version, commit, date)},
	)
	kctx.FatalIfErrorf(kctx.Run())
}

func (c *CompileConfig) Run() error {
	fds, err := compiler.Compile(c.Files, c.ProtoPath, c.IncludeImports)
	if err != nil {
		return err
	}
	b, err := proto.Marshal(fds)
	if err != nil {
		return err
	}
	f, err := os.Create(c.DescriptorSetOut)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	return err
}

func (c *CompileConfig) AfterApply() error {
	if len(c.Files) == 0 {
		return fmt.Errorf(`missing .proto input file(s)`)
	}
	return nil
}
