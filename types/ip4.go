/*
 * NETCAP - Traffic Analysis Framework
 * Copyright (c) 2017 Philipp Mieden <dreadl0ck [at] protonmail [dot] ch>
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
 * ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF
 * OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
 */

package types

import (
	"encoding/hex"
	"strconv"
	"strings"
)

func (i IPv4) CSVHeader() []string {
	return filter([]string{
		"Timestamp",
		"Version",        // int32
		"IHL",            // int32
		"TOS",            // int32
		"Length",         // int32
		"Id",             // int32
		"Flags",          // int32
		"FragOffset",     // int32
		"TTL",            // int32
		"Protocol",       // int32
		"Checksum",       // int32
		"SrcIP",          // string
		"DstIP",          // string
		"Padding",        // []byte
		"Options",        // []*IPv4Option
		"PayloadEntropy", // float64
		"PayloadSize",    // int32
	})
}

func (i IPv4) CSVRecord() []string {
	var opts []string
	for _, o := range i.Options {
		opts = append(opts, o.ToString())
	}
	return filter([]string{
		formatTimestamp(i.Timestamp),
		formatInt32(i.Version),        // int32
		formatInt32(i.IHL),            // int32
		formatInt32(i.TOS),            // int32
		formatInt32(i.Length),         // int32
		formatInt32(i.Id),             // int32
		formatInt32(i.Flags),          // int32
		formatInt32(i.FragOffset),     // int32
		formatInt32(i.TTL),            // int32
		formatInt32(i.Protocol),       // int32
		formatInt32(i.Checksum),       // int32
		i.SrcIP,                       // string
		i.DstIP,                       // string
		hex.EncodeToString(i.Padding), // []byte
		strings.Join(opts, ""),        // []*IPv4Option
		strconv.FormatFloat(i.PayloadEntropy, 'f', 6, 64), // float64
		formatInt32(i.PayloadSize),                        // int32
	})
}

func (i IPv4) NetcapTimestamp() string {
	return i.Timestamp
}

func (i IPv4Option) ToString() string {

	var b strings.Builder
	b.WriteString(Begin)
	b.WriteString(formatInt32(i.OptionType))
	b.WriteString(Separator)
	b.WriteString(formatInt32(i.OptionLength))
	b.WriteString(Separator)
	b.WriteString(hex.EncodeToString(i.OptionData))
	b.WriteString(End)

	return b.String()
}
