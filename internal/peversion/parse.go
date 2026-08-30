package peversion

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

const ffiSig = 0xFEEF04BD

func fromPE(f *pe.File) (string, error) {
	raw, err := io.ReadAll(sectionReader(f))
	if err != nil {
		return "", err
	}

	if v := stringAfterUTF16Key(raw, "ProductVersion"); v != "" {
		return v, nil
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
		patch := ls >> 16 // drop the fourth WORD (PE build / git REVISION)

		return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
	}

	return "", errors.New("no PE version resource")
}

func stringAfterUTF16Key(raw []byte, key string) string {
	pat := utf16z(key)
	i := bytes.Index(raw, pat)
	if i < 0 {
		return ""
	}

	p := i + len(pat)
	if p%2 == 1 {
		p++
	}

	for p+1 < len(raw) {
		start := p
		for p+1 < len(raw) {
			u := binary.LittleEndian.Uint16(raw[p:])
			p += 2
			if u == 0 {
				break
			}
		}

		n := (p - start - 2) / 2
		if n <= 0 {
			continue
		}

		u16 := make([]uint16, n)
		for j := range n {
			u16[j] = binary.LittleEndian.Uint16(raw[start+j*2:])
		}

		s := strings.TrimSpace(string(utf16.Decode(u16)))
		if s == "" {
			continue
		}

		if s[0] >= '0' && s[0] <= '9' {
			return s
		}

		return ""
	}

	return ""
}

func utf16z(s string) []byte {
	b := make([]byte, 0, len(s)*2+2)
	for i := range len(s) {
		b = append(b, s[i], 0)
	}

	return append(b, 0, 0)
}
