package model

import "bytes"

func RemoveDuplicatedRunes(s string, rep rune) string {
	var buf bytes.Buffer
	var last rune
	for i, r := range s {
		if r != last || i == 0 {
			buf.WriteRune(r)
			last = r
		} else if r != rep {
			buf.WriteRune(r)
			last = r
		}
	}
	return buf.String()
}

func StripNonAscii(s []byte) []byte {
	n := 0
	for _, b := range s {
		if ('a' <= b && b <= 'z') ||
			('A' <= b && b <= 'Z') ||
			('0' <= b && b <= '9') || b == ' ' {
			s[n] = b
			n++
		}
	}
	return s[:n]
}

func ReplaceSpaces(s []byte, r byte) []byte {
	for i, b := range s {
		if b == ' ' {
			s[i] = r
		}
	}
	return s
}
