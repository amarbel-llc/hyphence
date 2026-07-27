package commands_hyphence

import (
	"io"
	"os"

	"code.linenisgreat.com/hyphence/go/futility"
	"code.linenisgreat.com/hyphence/go/hyphence"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/errors"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/interfaces"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/values"
)

func init() {
	utility.AddCmd("validate", &Validate{})
}

type Validate struct{}

var _ futility.CommandWithParams = (*Validate)(nil)

func (cmd *Validate) GetParams() []futility.Param {
	return []futility.Param{
		futility.Arg[*values.String]{
			Name:        "path",
			Description: "path to a hyphence document, or '-' for stdin",
			Required:    true,
		},
	}
}

func (cmd Validate) GetDescription() futility.Description {
	return futility.Description{
		Short: "strict RFC 0001 conformance check",
		Long: "Read a hyphence document and verify it conforms to RFC " +
			"0001. Exits 0 silent on pass; exits 65 with one line- " +
			"numbered diagnostic on stderr on the first violation. " +
			"Validate also enforces the inline-body-AND-@ rule (RFC " +
			"0001 §Metadata Lines): a document MUST NOT carry both an " +
			"@ blob-reference line and a body section. Each line's " +
			"content is additionally checked against the RFC 0002/0003 " +
			"content grammar, which includes the charset-strict markl-id " +
			"digest slot: a digest payload outside the blech32 alphabet " +
			"(such as blake2b256-9bt3) is rejected. Note the library's " +
			"own decoders stay envelope-only and accept such content, so " +
			"this stricter check is what `validate` adds over decoding.",
	}
}

func (cmd *Validate) SetFlagDefinitions(interfaces.CLIFlagDefinitions) {}

func (cmd Validate) Run(req futility.Request) {
	path := req.PopArg("path")
	req.AssertNoMoreArgs()

	in, source, closer, err := OpenInput(path, os.Stdin)
	if err != nil {
		bail(req, "validate", path, err)
		return
	}
	defer errors.ContextMustClose(req, closer)

	if err := validateDocument(in); err != nil {
		bail(req, "validate", source, err)
		return
	}
}

// validateDocument runs the strict-RFC validation pipeline against in
// and returns the first violation, or nil on success. Shared between
// Validate.Run (which prints diagnostics and cancels the request) and
// the test seam in validate_test.go.
func validateDocument(in io.Reader) error {
	// CheckContent adds the RFC 0002/0003 content-grammar pass on top of
	// RFC 0001 envelope conformance (hyphence#11). It is opt-in precisely
	// so the strictness lands here, on the subcommand whose job is strict
	// conformance, and not on the library's decode path — see
	// MetadataValidator.CheckContent for why defaulting it on would break
	// the retired-spelling compatibility vectors.
	v := &hyphence.MetadataValidator{CheckContent: true}
	body := &hyphence.CountingDiscardReaderFrom{}
	reader := hyphence.Reader{
		RequireMetadata: true,
		Metadata:        v,
		Blob:            body,
	}
	if _, err := reader.ReadFrom(in); err != nil {
		return err
	}
	if v.SawAtLine && body.SawBody {
		return hyphence.ErrInlineBodyWithAtReference
	}
	return nil
}
