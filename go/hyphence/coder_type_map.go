package hyphence

import (
	"bufio"

	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/errors"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/interfaces"
)

// typePtr / digestPtr are pointer-receiver constraints. The typed coders call
// Set (a pointer-receiver method on the type-tag / digest), so they must operate
// through *T / *D, not the value types T / D — hence the PT / PD constraints that
// pin the pointer and the method set. They instantiate to *Type / *Digest by
// default (see the *Default aliases below); madder instantiates them with
// *ids.TypeStruct / *markl.Id.
type typePtr[T any] interface {
	*T
	Set(string) error
	String() string
	StringSansOp() string
}

type digestPtr[D any] interface {
	*D
	Set(string) error
	String() string
	IsNull() bool
}

// TypedBlob is the hyphence typed-document model: a type tag, an optional blob
// digest, and a typed blob body. It is generic over the type-tag type (T, with
// pointer constraint PT) and the digest type (D, with pointer constraint PD) so
// a consumer can plug in its own native types — e.g. madder's ids.TypeStruct /
// markl.Id, which carry markl-specific methods the opaque string types cannot.
// This package's opaque Type / Digest are the default instantiation; see
// TypedBlobDefault.
type TypedBlob[T any, PT typePtr[T], D any, PD digestPtr[D], BLOB any] struct {
	Type       T
	BlobDigest D
	Blob       BLOB
}

// The *Default aliases bind T/PT/D/PD to this package's opaque Type / Digest, so
// standalone callers and the conformance suite instantiate with a single BLOB
// type parameter exactly as before the generic split.
type (
	TypedBlobDefault[BLOB any]                 = TypedBlob[Type, *Type, Digest, *Digest, BLOB]
	TypedMetadataCoderDefault[BLOB any]        = TypedMetadataCoder[Type, *Type, Digest, *Digest, BLOB]
	CoderTypeMapDefault[BLOB any]              = CoderTypeMap[Type, *Type, Digest, *Digest, BLOB]
	CoderTypeMapWithoutTypeDefault[BLOB any]   = CoderTypeMapWithoutType[Type, *Type, Digest, *Digest, BLOB]
	DecoderTypeMapWithoutTypeDefault[BLOB any] = DecoderTypeMapWithoutType[Type, *Type, Digest, *Digest, BLOB]
	EncoderTypeMapWithoutTypeDefault[BLOB any] = EncoderTypeMapWithoutType[Type, *Type, Digest, *Digest, BLOB]
	CoderToTypedBlobDefault[BLOB any]          = CoderToTypedBlob[Type, *Type, Digest, *Digest, BLOB]
)

type TypedBlobEmpty = TypedBlobDefault[struct{}]

type CoderTypeMap[T any, PT typePtr[T], D any, PD digestPtr[D], BLOB any] map[string]interfaces.CoderBufferedReadWriter[*TypedBlob[T, PT, D, PD, BLOB]]

func (coderTypeMap CoderTypeMap[T, PT, D, PD, BLOB]) DecodeFrom(
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	bufferedReader *bufio.Reader,
) (n int64, err error) {
	tipe := PT(&typedBlob.Type)
	coder, ok := coderTypeMap[tipe.String()]

	if !ok {
		err = errors.ErrorWithStackf("no coders available for type: %q", tipe)
		return n, err
	}

	if n, err = coder.DecodeFrom(typedBlob, bufferedReader); err != nil {
		err = errors.Wrap(err)
		return n, err
	}

	return n, err
}

func (coderTypeMap CoderTypeMap[T, PT, D, PD, BLOB]) EncodeTo(
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	bufferedWriter *bufio.Writer,
) (n int64, err error) {
	tipe := PT(&typedBlob.Type)
	coder, ok := coderTypeMap[tipe.String()]

	if !ok {
		err = errors.ErrorWithStackf("no coders available for type: %q", tipe)
		return n, err
	}

	if n, err = coder.EncodeTo(typedBlob, bufferedWriter); err != nil {
		err = errors.Wrap(err)
		return n, err
	}

	return n, err
}

type DecoderTypeMapWithoutType[T any, PT typePtr[T], D any, PD digestPtr[D], BLOB any] map[string]interfaces.DecoderFromBufferedReader[BLOB]

func (decoderTypeMap DecoderTypeMapWithoutType[T, PT, D, PD, BLOB]) DecodeFrom(
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	bufferedReader *bufio.Reader,
) (n int64, err error) {
	tipe := PT(&typedBlob.Type)
	decoder, ok := decoderTypeMap[tipe.String()]

	if !ok {
		err = errors.ErrorWithStackf("no coders available for type: %q", tipe)
		return n, err
	}

	if n, err = decoder.DecodeFrom(
		typedBlob.Blob,
		bufferedReader,
	); err != nil {
		err = errors.Wrap(err)
		return n, err
	}

	return n, err
}

type CoderTypeMapWithoutType[T any, PT typePtr[T], D any, PD digestPtr[D], BLOB any] map[string]interfaces.CoderBufferedReadWriter[*BLOB]

func (coderTypeMap CoderTypeMapWithoutType[T, PT, D, PD, BLOB]) DecodeFrom(
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	bufferedReader *bufio.Reader,
) (n int64, err error) {
	tipe := PT(&typedBlob.Type)
	coder, ok := coderTypeMap[tipe.String()]

	if !ok {
		err = errors.ErrorWithStackf("no coders available for type: %q", tipe)
		return n, err
	}

	if n, err = coder.DecodeFrom(&typedBlob.Blob, bufferedReader); err != nil {
		err = errors.Wrap(err)
		return n, err
	}

	return n, err
}

func (coderTypeMap CoderTypeMapWithoutType[T, PT, D, PD, BLOB]) EncodeTo(
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	bufferedWriter *bufio.Writer,
) (n int64, err error) {
	tipe := PT(&typedBlob.Type)
	coder, ok := coderTypeMap[tipe.String()]

	if !ok {
		err = errors.ErrorWithStackf("no coders available for type: %q", tipe)
		return n, err
	}

	if n, err = coder.EncodeTo(&typedBlob.Blob, bufferedWriter); err != nil {
		err = errors.Wrap(err)
		return n, err
	}

	return n, err
}
