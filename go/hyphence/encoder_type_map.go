package hyphence

import (
	"bufio"

	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/errors"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/interfaces"
)

type EncoderTypeMapWithoutType[T any, PT typePtr[T], D any, PD digestPtr[D], BLOB any] map[string]interfaces.EncoderToBufferedWriter[BLOB]

func (coderTypeMap EncoderTypeMapWithoutType[T, PT, D, PD, BLOB]) EncodeTo(
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	bufferedWriter *bufio.Writer,
) (n int64, err error) {
	tipe := PT(&typedBlob.Type)
	coder, ok := coderTypeMap[tipe.String()]

	if !ok {
		err = errors.ErrorWithStackf("no coders available for type: %q", tipe)
		return n, err
	}

	if n, err = coder.EncodeTo(typedBlob.Blob, bufferedWriter); err != nil {
		err = errors.Wrap(err)
		return n, err
	}

	return n, err
}
