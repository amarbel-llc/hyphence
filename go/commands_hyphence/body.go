package commands_hyphence

import (
	"os"

	"code.linenisgreat.com/hyphence/go/futility"
	"code.linenisgreat.com/hyphence/go/hyphence"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/errors"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/interfaces"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/values"
)

func init() {
	utility.AddCmd("body", &Body{})
}

type Body struct{}

var _ futility.CommandWithParams = (*Body)(nil)

func (cmd *Body) GetParams() []futility.Param {
	return []futility.Param{
		futility.Arg[*values.String]{
			Name:        "path",
			Description: "path to a hyphence document, or '-' for stdin",
			Required:    true,
		},
	}
}

func (cmd Body) GetDescription() futility.Description {
	return futility.Description{
		Short: "print body section verbatim",
		Long: "Read a hyphence document and stream its body section " +
			"(the bytes after the closing --- and the body separator) " +
			"to stdout. If the document has no body, prints nothing and " +
			"exits 0.",
	}
}

func (cmd *Body) SetFlagDefinitions(interfaces.CLIFlagDefinitions) {}

func (cmd Body) Run(req futility.Request) {
	path := req.PopArg("path")
	req.AssertNoMoreArgs()

	in, source, closer, err := OpenInput(path, os.Stdin)
	if err != nil {
		bail(req, "body", path, err)
		return
	}
	defer errors.ContextMustClose(req, closer)

	body := &hyphence.BodyStreamer{W: os.Stdout}
	reader := hyphence.Reader{
		RequireMetadata: true,
		Metadata:        &hyphence.CountingDiscardReaderFrom{},
		Blob:            body,
	}

	if _, err := reader.ReadFrom(in); err != nil {
		bail(req, "body", source, err)
		return
	}
}
