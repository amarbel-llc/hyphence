package commands_hyphence

import (
	"os"

	"code.linenisgreat.com/hyphence/go/futility"
	"code.linenisgreat.com/hyphence/go/hyphence"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/errors"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/interfaces"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/values"
)

func init() {
	utility.AddCmd("format", &Format{})
}

type Format struct{}

var _ futility.CommandWithParams = (*Format)(nil)

func (cmd *Format) GetParams() []futility.Param {
	return []futility.Param{
		futility.Arg[*values.String]{
			Name:        "path",
			Description: "path to a hyphence document, or '-' for stdin",
			Required:    true,
		},
	}
}

func (cmd Format) GetDescription() futility.Description {
	return futility.Description{
		Short: "re-emit canonicalized per RFC §Canonical Line Order",
		Long: "Read a hyphence document and re-emit it with metadata " +
			"lines sorted per RFC 0002 §`<` Deprecation (amending RFC " +
			"0001 §Canonical Line Order): description (#) → " +
			"reference, tag, and field lines (-, including the " +
			"deprecated < synonym) → blob reference (@) → type (!). " +
			"Within the -/< bucket, source order is preserved. " +
			"Comments (%) stay anchored to their following non-comment " +
			"line. Body bytes pass through unchanged.",
	}
}

func (cmd *Format) SetFlagDefinitions(interfaces.CLIFlagDefinitions) {}

func (cmd Format) Run(req futility.Request) {
	path := req.PopArg("path")
	req.AssertNoMoreArgs()

	in, source, closer, err := OpenInput(path, os.Stdin)
	if err != nil {
		bail(req, "format", path, err)
		return
	}
	defer errors.ContextMustClose(req, closer)

	doc := &hyphence.Document{}
	builder := &hyphence.MetadataBuilder{Doc: doc}
	emitter := &hyphence.FormatBodyEmitter{Doc: doc, Out: os.Stdout}
	reader := hyphence.Reader{
		RequireMetadata: true,
		Metadata:        builder,
		Blob:            emitter,
	}

	if _, err := reader.ReadFrom(in); err != nil {
		bail(req, "format", source, err)
		return
	}
}
