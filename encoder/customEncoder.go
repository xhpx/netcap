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

package encoder

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/dreadl0ck/netcap"
	"github.com/dreadl0ck/netcap/types"
	"github.com/golang/protobuf/proto"
	"github.com/google/gopacket"
	"kythe.io/kythe/go/platform/delimited"
)

var (
	// CustomEncoders slice contains initialized encoders at runtime
	// for usage from other packages
	CustomEncoders = []*CustomEncoder{}

	// contains all available custom encoders
	customEncoderSlice = []*CustomEncoder{
		tlsEncoder,
		linkFlowEncoder,
		networkFlowEncoder,
		transportFlowEncoder,
		httpEncoder,
		flowEncoder,
		connectionEncoder,
	}
)

type (
	// CustomEncoderHandler takes a gopacket.Packet and returns a proto.Message
	CustomEncoderHandler = func(p gopacket.Packet) proto.Message

	// CustomEncoder implements custom logic to decode data from a gopacket.Packet
	CustomEncoder struct {

		// public fields
		Name string
		Type types.Type

		// private fields
		file      *os.File
		bWriter   *bufio.Writer
		gWriter   *gzip.Writer
		dWriter   *delimited.Writer
		aWriter   *AtomicDelimitedWriter
		cWriter   *chanWriter
		csvWriter *csvWriter

		Handler  CustomEncoderHandler
		postinit func(*CustomEncoder) error
		deinit   func(*CustomEncoder) error

		// configuration
		compress bool
		buffer   bool
		csv      bool
		out      string

		// used to keep track of the number of generated audit records
		numRecords int64
	}
)

// package level init
func init() {
	// collect all names for custom encoders on startup
	for _, e := range customEncoderSlice {
		allEncoderNames[e.Name] = struct{}{}
	}
	// collect all names for custom encoders on startup
	for _, e := range layerEncoderSlice {
		allEncoderNames[e.Layer.String()] = struct{}{}
	}
}

// InitCustomEncoders initializes all custom encoders
func InitCustomEncoders(c Config) {

	var (
		// values from command-line flags
		in = strings.Split(c.IncludeEncoders, ",")
		ex = strings.Split(c.ExcludeEncoders, ",")

		// include map
		inMap = make(map[string]bool)

		// new selection
		selection []*CustomEncoder
	)

	// if there are includes and the first item is not an empty string
	if len(in) > 0 && in[0] != "" {

		// iterate over includes
		for _, name := range in {
			if name != "" {

				// check if proto exists
				if _, ok := allEncoderNames[name]; !ok {
					invalidEncoder(name)
				}

				// add to include map
				inMap[name] = true
			}
		}

		// iterate over custom encoders and collect those that are named in the includeMap
		for _, e := range customEncoderSlice {
			if _, ok := inMap[e.Name]; ok {
				selection = append(selection, e)
			}
		}

		// update custom encoders to new selection
		customEncoderSlice = selection
	}

	// iterate over excluded encoders
	for _, name := range ex {
		if name != "" {

			// check if proto exists
			if _, ok := allEncoderNames[name]; !ok {
				invalidEncoder(name)
			}

			// remove named encoder from customEncoderSlice
			for i, e := range customEncoderSlice {
				if name == e.Name {
					// remove encoder
					customEncoderSlice = append(customEncoderSlice[:i], customEncoderSlice[i+1:]...)
					break
				}
			}
		}
	}

	// initialize encoders
	for _, e := range customEncoderSlice {

		// fmt.Println("init custom encoder", e.name)
		e.Init(c.Buffer, c.Compression, c.CSV, c.Out, c.WriteChan)

		// call postinit func if set
		if e.postinit != nil {
			err := e.postinit(e)
			if err != nil {
				panic(err)
			}
		}

		// write header
		if e.csv {
			_, err := e.csvWriter.WriteHeader(netcap.InitRecord(e.Type))
			if err != nil {
				panic(err)
			}
		} else {
			err := e.aWriter.PutProto(NewHeader(e.Type, c))
			if err != nil {
				fmt.Println("failed to write header")
				panic(err)
			}
		}

		// append to custom encoders slice
		CustomEncoders = append(CustomEncoders, e)
	}
	fmt.Println("initialized", len(CustomEncoders), "custom encoders | buffer size:", BlockSize)
}

// CreateCustomEncoder returns a new CustomEncoder instance
func CreateCustomEncoder(t types.Type, name string, postinit func(*CustomEncoder) error, handler CustomEncoderHandler, deinit func(*CustomEncoder) error) *CustomEncoder {
	return &CustomEncoder{
		Name:     name,
		Handler:  handler,
		deinit:   deinit,
		postinit: postinit,
		Type:     t,
	}
}

// Encode is called for each layer
// this calls the handler function of the encoder
// and writes the serialized protobuf into the data pipe
func (e *CustomEncoder) Encode(p gopacket.Packet) error {

	// call the Handler function of the encoder
	decoded := e.Handler(p)
	if decoded != nil {

		// increase counter
		atomic.AddInt64(&e.numRecords, 1)

		// write record
		err := e.aWriter.PutProto(decoded)
		if err != nil {
			return err
		}
	}
	return nil
}

// Init initializes and configures the encoder
func (e *CustomEncoder) Init(buffer, compress, csv bool, out string, writeChan bool) {

	e.compress = compress
	e.buffer = buffer
	e.csv = csv
	e.out = out

	if csv {

		// create file
		if compress {
			e.file = CreateFile(filepath.Join(out, e.Name), ".csv.gz")
		} else {
			e.file = CreateFile(filepath.Join(out, e.Name), ".csv")
		}

		if buffer {

			e.bWriter = bufio.NewWriterSize(e.file, BlockSize)

			if compress {
				e.gWriter = gzip.NewWriter(e.bWriter)
				e.csvWriter = NewCSVWriter(e.gWriter)
			} else {
				e.csvWriter = NewCSVWriter(e.bWriter)
			}
		} else {
			if compress {
				e.gWriter = gzip.NewWriter(e.file)
				e.csvWriter = NewCSVWriter(e.gWriter)
			} else {
				e.csvWriter = NewCSVWriter(e.file)
			}
		}
		return
	}

	if writeChan && buffer || writeChan && compress {
		panic("buffering or compression cannot be activated when running using writeChan")
	}

	// write into channel OR into file
	if writeChan {
		e.cWriter = newChanWriter()
	} else {
		if compress {
			e.file = CreateFile(filepath.Join(out, e.Name), ".ncap.gz")
		} else {
			e.file = CreateFile(filepath.Join(out, e.Name), ".ncap")
		}
	}

	// buffer data?
	if buffer {

		e.bWriter = bufio.NewWriterSize(e.file, BlockSize)
		if compress {
			e.gWriter = gzip.NewWriter(e.bWriter)
			e.dWriter = delimited.NewWriter(e.gWriter)
		} else {
			e.dWriter = delimited.NewWriter(e.bWriter)
		}
	} else {
		if compress {
			e.gWriter = gzip.NewWriter(e.file)
			e.dWriter = delimited.NewWriter(e.gWriter)
		} else {
			if writeChan {
				// write into channel writer without compression
				e.dWriter = delimited.NewWriter(e.cWriter)
			} else {
				e.dWriter = delimited.NewWriter(e.file)
			}
		}
	}
	e.aWriter = NewAtomicDelimitedWriter(e.dWriter)
}

// Destroy closes and flushes all writers and calls deinit if set
func (e *CustomEncoder) Destroy() (name string, size int64) {
	if e.deinit != nil {
		err := e.deinit(e)
		if err != nil {
			panic(err)
		}
	}
	if e.compress {
		CloseGzipWriters(e.gWriter)
	}
	if e.buffer {
		FlushWriters(e.bWriter)
	}
	return CloseFile(e.out, e.file, e.Name)
}

// GetChan returns a channel to receive serialized protobuf data from the encoder
func (e *CustomEncoder) GetChan() <-chan []byte {
	return e.cWriter.Chan()
}

func (e *CustomEncoder) NumRecords() int64 {
	return e.numRecords
}
