package hyphence

import (
	"bufio"
	"errors"
	"io"
	"strings"
	"testing"
)

// Every state machine that reads a metadata section must refuse input that
// ends before the closing boundary (RFC 0001 §Conformance), as Reader does.

var unterminatedMetadataInputs = []string{
	"---\n",
	"---\n! toml-type-v1\n",
	"---\n! toml-type-v1",
}

const terminatedMetadataWithoutBody = "---\n! toml-type-v1\n---\n"

func decodeWithDecoder(input string) error {
	decoder := Decoder[*TypedBlobDefault[struct{}]]{
		RequireMetadata: true,
		Metadata:        TypedMetadataCoderDefault[struct{}]{},
		Blob:            testBlobDecoder{},
	}

	_, err := decoder.DecodeFrom(
		&TypedBlobDefault[struct{}]{},
		bufio.NewReader(strings.NewReader(input)),
	)

	return err
}

func decodeWithCoder(input string) error {
	coder := Coder[*TypedBlobDefault[struct{}]]{
		RequireMetadata: true,
		Metadata:        TypedMetadataCoderDefault[struct{}]{},
		Blob:            TypedMetadataCoderDefault[struct{}]{},
	}

	var reader io.Reader = strings.NewReader(input)

	_, err := coder.readMetadataFrom(&TypedBlobDefault[struct{}]{}, &reader)

	return err
}

func decodeWithCoderToTypedBlob(input string) error {
	coder := CoderToTypedBlobDefault[struct{}]{
		RequireMetadata: true,
		Metadata:        TypedMetadataCoderDefault[struct{}]{},
		Blob:            CoderTypeMapWithoutType[Type, *Type, Digest, *Digest, struct{}]{},
	}

	var reader io.Reader = strings.NewReader(input)

	_, err := coder.readMetadataFrom(&TypedBlobDefault[struct{}]{}, &reader)

	return err
}

var metadataDecoders = map[string]func(string) error{
	"Decoder":          decodeWithDecoder,
	"Coder":            decodeWithCoder,
	"CoderToTypedBlob": decodeWithCoderToTypedBlob,
}

func TestDecodersRefuseUnterminatedMetadata(t *testing.T) {
	for name, decode := range metadataDecoders {
		for _, input := range unterminatedMetadataInputs {
			if err := decode(input); !errors.Is(err, ErrUnterminatedMetadata) {
				t.Errorf("%s %q: err = %v, want ErrUnterminatedMetadata", name, input, err)
			}
		}
	}
}

func TestDecodersAcceptClosingBoundaryAtEOF(t *testing.T) {
	for name, decode := range metadataDecoders {
		if err := decode(terminatedMetadataWithoutBody); err != nil {
			t.Errorf("%s %q: unexpected err %v", name, terminatedMetadataWithoutBody, err)
		}
	}
}
