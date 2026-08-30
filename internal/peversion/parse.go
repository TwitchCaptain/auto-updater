package peversion

import (
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const ffiSig = 0xFEEF04BD

func fromPE(f *pe.File) (string, error) {
	raw, err := io.ReadAll(sectionReader(f))
	if err != nil {
		return "", err
	}

	return parseFixed(raw)
}

func sectionReader(f *pe.File) io.Reader {
	// Concatenate all sections; VERSIONINFO lives in .rsrc.
	readers := make([]io.Reader, 0, len(f.Sections))
	for _, s := range f.Sections {
		readers = append(readers, s.Open())
	}

	if len(readers) == 0 {
		return io.MultiReader()
	}

	return io.MultiReader(readers...)
}

func parseFixed(raw []byte) (string, error) {
	for i := 0; i+24 <= len(raw); i += 4 {
		if binary.LittleEndian.Uint32(raw[i:]) != ffiSig {
			continue
		}

		if i+24 > len(raw) {
			break
		}

		ms := binary.LittleEndian.Uint32(raw[i+8:])
		ls := binary.LittleEndian.Uint32(raw[i+12:])
		major := ms >> 16
		minor := ms & 0xffff
		patch := ls >> 16
		build := ls & 0xffff
		if build == 0 {
			return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
		}

		return fmt.Sprintf("%d.%d.%d.%d", major, minor, patch, build), nil
	}

	return "", errors.New("no PE version resource")
}
