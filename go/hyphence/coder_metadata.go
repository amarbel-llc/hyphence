package hyphence

import (
	"bufio"
	"fmt"

	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/errors"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/format"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/interfaces"
	"code.linenisgreat.com/purse-first/libs/dewey/pkgs/ohio"
)

type TypedMetadataCoder[T any, PT typePtr[T], D any, PD digestPtr[D], BLOB any] struct{}

func (TypedMetadataCoder[T, PT, D, PD, BLOB]) DecodeFrom(
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	bufferedReader *bufio.Reader,
) (n int64, err error) {
	// TODO scan for type directly
	if n, err = format.ReadLines(
		bufferedReader,
		ohio.MakeLineReaderRepeat(
			ohio.MakeLineReaderKeyValues(
				map[string]interfaces.FuncSetString{
					"!": PT(&typedBlob.Type).Set,
					"@": PD(&typedBlob.BlobDigest).Set,
				},
			),
		),
	); err != nil {
		err = errors.Wrap(err)
		return n, err
	}

	return n, err
}

func (TypedMetadataCoder[T, PT, D, PD, BLOB]) EncodeTo(
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	bufferedWriter *bufio.Writer,
) (n int64, err error) {
	var n1 int

	n1, err = fmt.Fprintf(
		bufferedWriter,
		"! %s\n",
		PT(&typedBlob.Type).StringSansOp(),
	)
	n += int64(n1)

	if err != nil {
		err = errors.Wrap(err)
		return n, err
	}

	if !PD(&typedBlob.BlobDigest).IsNull() {
		n1, err = fmt.Fprintf(
			bufferedWriter,
			"@ %s\n",
			PD(&typedBlob.BlobDigest),
		)
		n += int64(n1)

		if err != nil {
			err = errors.Wrap(err)
			return n, err
		}
	}

	return n, err
}
