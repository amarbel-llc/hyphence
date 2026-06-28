package hyphence

import (
	"bytes"
	"os"

	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/errors"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/files"
	"github.com/amarbel-llc/purse-first/libs/dewey/pkgs/interfaces"
)

func DecodeFromFileInto[
	T any,
	PT typePtr[T],
	D any,
	PD digestPtr[D],
	BLOB any,
	BLOB_PTR interfaces.Ptr[BLOB],
](
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	coders CoderToTypedBlob[T, PT, D, PD, BLOB],
	path string,
) (err error) {
	var file *os.File

	if path == "-" {
		file = os.Stdin
	} else {
		if file, err = files.OpenExclusiveReadOnly(
			path,
		); err != nil {
			err = errors.Wrap(err)
			return err
		}

		defer errors.DeferredCloser(&err, file)
	}

	if _, err = coders.DecodeFrom(typedBlob, file); err != nil {
		err = errors.Wrap(err)
		return err
	}

	return err
}

func DecodeFromFile[
	T any,
	PT typePtr[T],
	D any,
	PD digestPtr[D],
	BLOB any,
	BLOB_PTR interfaces.Ptr[BLOB],
](
	coders CoderToTypedBlob[T, PT, D, PD, BLOB],
	path string,
) (typedBlob TypedBlob[T, PT, D, PD, BLOB], err error) {
	if err = DecodeFromFileInto(&typedBlob, coders, path); err != nil {
		err = errors.Wrap(err)
		return typedBlob, err
	}

	return typedBlob, err
}

// EncodeToFile writes typedBlob through coders.EncodeTo to path. A path
// of "-" routes to stdout, mirroring DecodeFromFileInto's stdin
// shortcut. For real paths, files.CreateExclusiveWriteOnly is used so a
// concurrent writer fails fast rather than producing a torn write.
func EncodeToFile[
	T any,
	PT typePtr[T],
	D any,
	PD digestPtr[D],
	BLOB any,
	BLOB_PTR interfaces.Ptr[BLOB],
](
	coders CoderToTypedBlob[T, PT, D, PD, BLOB],
	typedBlob *TypedBlob[T, PT, D, PD, BLOB],
	path string,
) (err error) {
	var file *os.File

	if path == "-" {
		file = os.Stdout
	} else {
		if file, err = files.CreateExclusiveWriteOnly(
			path,
		); err != nil {
			err = errors.Wrap(err)
			return err
		}

		defer errors.DeferredCloser(&err, file)
	}

	if _, err = coders.EncodeTo(typedBlob, file); err != nil {
		err = errors.Wrap(err)
		return err
	}

	return err
}

// For decodes where typedBlob.Blob should be populated with an empty struct
// when the file is missing
func DecodeFromFileOrEmptyBuffer[
	T any,
	PT typePtr[T],
	D any,
	PD digestPtr[D],
	BLOB any,
	BLOB_PTR interfaces.Ptr[BLOB],
](
	coders CoderToTypedBlob[T, PT, D, PD, BLOB],
	path string,
	permitNotExist bool,
) (typedBlob TypedBlob[T, PT, D, PD, BLOB], err error) {
	typedBlob, err = DecodeFromFile(coders, path)

	if err == nil {
		return typedBlob, err
	}

	if _, err = coders.DecodeFrom(&typedBlob, bytes.NewBuffer(nil)); err != nil {
		err = errors.Wrap(err)
		return typedBlob, err
	}

	return typedBlob, err
}
