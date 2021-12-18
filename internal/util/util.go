package util

import (
	"bytes"
	"math/rand"
	"net"
	"net/url"
	"path"
	"time"
)

// URLJoin receives url path components, joins them just like strings.Join but for URL path.
func URLJoin(baseURL string, subPath ...string) string {
	u, _ := url.Parse(baseURL)
	p := []string{u.Path}
	p = append(p, subPath...)
	u.Path = path.Join(p...)
	return u.String()
}

var seededRand *rand.Rand = rand.New(
	rand.NewSource(time.Now().UnixNano()))

// RandStringWithCharset receives length of random string, and sets of character to combine them, then returns output.
func RandStringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

const digitCharset = "0123456789"
const charset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ" + digitCharset

// RandDigitString receives length of random string, then returns with digital string (0-9)
func RandDigitString(length int) string {
	return RandStringWithCharset(length, digitCharset)
}

// RandString receives length of random string, then returns with ascii string (0-9a-zA-Z)
func RandString(length int) string {
	return RandStringWithCharset(length, charset)
}

// MacUInt64 get mac address and chop it into uint64
func MacUInt64() uint64 {
	interfaces, err := net.Interfaces()
	if nil != err {
		return uint64(0)
	}

	for _, i := range interfaces {
		if 0 != i.Flags&net.FlagUp &&
			0 != bytes.Compare(i.HardwareAddr, nil) {
			// Skip locally administered addresses
			if 2 == i.HardwareAddr[0]&2 {
				continue
			}

			var mac uint64
			for j, b := range i.HardwareAddr {
				if j >= 8 {
					break
				}
				mac <<= 8
				mac += uint64(b)
			}

			return mac
		}
	}

	return uint64(0)
}
