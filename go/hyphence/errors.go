package hyphence

import "code.linenisgreat.com/purse-first/libs/dewey/pkgs/errors"

type (
	pkgErrDisamb struct{}
	pkgError     = errors.Typed[pkgErrDisamb]
)

func newPkgError(text string) pkgError {
	return errors.NewWithType[pkgErrDisamb](text)
}

var errMissingNewlineAfterBoundary = newPkgError(
	"missing blank line after closing boundary --- before blob body",
)

// ErrUnterminatedMetadata is returned when input ends inside the metadata
// section: an opening boundary with no closing one. RFC 0001 requires EOF
// mid-document to be a parse error; Rust's twin is
// Error::UnterminatedMetadata.
var ErrUnterminatedMetadata = newPkgError(
	"unterminated metadata section (no closing boundary ---)",
)
