package main

import (
	"bytes"
	"encoding/hex"
)

func parseString(b []byte) string {
	n := 0
	for n < len(b) && b[n] != 0 {
		n++
	}
	return string(b[0:n])
}

func stringify(a, b byte) string {
	return dumpHex(append([]byte{a}, b))
}

func dumpHex(data []byte) string {
	var buf bytes.Buffer
	var tmp [3]byte
	for i := range data {
		hex.Encode(tmp[:], data[i:i+1])
		tmp[2] = ' '
		_, err := buf.Write(tmp[:3])
		if err != nil {
			return ""
		}
	}
	return buf.String()
}
