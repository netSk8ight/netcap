package netcap

import (
	"bufio"
	"compress/gzip"
	"os"
	"path/filepath"

	"github.com/dreadl0ck/netcap/io"
	"github.com/dreadl0ck/netcap/types"
	"github.com/golang/protobuf/proto"
	"kythe.io/kythe/go/platform/delimited"
)

const blockSizeDefault = 4096

var (

	// BlockSize is the file system block size
	BlockSize int
)

/*
 *	Type Definition
 */

// Writer supports writing audit records to disk
type Writer struct {

	// Name of the associated audit record type
	Name string

	// private fields
	file      *os.File
	bWriter   *bufio.Writer
	gWriter   *gzip.Writer
	dWriter   *delimited.Writer
	aWriter   *io.AtomicDelimitedWriter
	cWriter   *io.ChanWriter
	csvWriter *io.CSVWriter

	// configuration
	compress     bool
	buffer       bool
	csv          bool
	out          string
	IsChanWriter bool
}

/*
 *	Constructor
 */

// NewWriter initializes and configures a new Writer
func NewWriter(name string, buffer, compress, csv bool, out string, writeChan bool) *Writer {

	w := &Writer{}
	w.Name = name
	w.compress = compress
	w.buffer = buffer
	w.csv = csv
	w.out = out
	w.IsChanWriter = writeChan

	if csv {

		// create file
		if compress {
			w.file = CreateFile(filepath.Join(out, w.Name), ".csv.gz")
		} else {
			w.file = CreateFile(filepath.Join(out, w.Name), ".csv")
		}

		if buffer {

			w.bWriter = bufio.NewWriterSize(w.file, BlockSize)

			if compress {
				w.gWriter = gzip.NewWriter(w.bWriter)
				w.csvWriter = io.NewCSVWriter(w.gWriter)
			} else {
				w.csvWriter = io.NewCSVWriter(w.bWriter)
			}
		} else {
			if compress {
				w.gWriter = gzip.NewWriter(w.file)
				w.csvWriter = io.NewCSVWriter(w.gWriter)
			} else {
				w.csvWriter = io.NewCSVWriter(w.file)
			}
		}
		return nil
	}

	if writeChan && buffer || writeChan && compress {
		panic("buffering or compression cannot be activated when running using writeChan")
	}

	// write into channel OR into file
	if writeChan {
		w.cWriter = io.NewChanWriter()
	} else {
		if compress {
			w.file = CreateFile(filepath.Join(out, w.Name), ".ncap.gz")
		} else {
			w.file = CreateFile(filepath.Join(out, w.Name), ".ncap")
		}
	}

	// buffer data?
	if buffer {

		w.bWriter = bufio.NewWriterSize(w.file, BlockSize)
		if compress {
			w.gWriter = gzip.NewWriter(w.bWriter)
			w.dWriter = delimited.NewWriter(w.gWriter)
		} else {
			w.dWriter = delimited.NewWriter(w.bWriter)
		}
	} else {
		if compress {
			w.gWriter = gzip.NewWriter(w.file)
			w.dWriter = delimited.NewWriter(w.gWriter)
		} else {
			if writeChan {
				// write into channel writer without compression
				w.dWriter = delimited.NewWriter(w.cWriter)
			} else {
				w.dWriter = delimited.NewWriter(w.file)
			}
		}
	}
	w.aWriter = io.NewAtomicDelimitedWriter(w.dWriter)

	return w
}

/*
 *	Protobuf
 */

// WriteProto writes a protobuf message
func (w *Writer) WriteProto(msg proto.Message) error {
	return w.aWriter.PutProto(msg)
}

/*
 *	CSV
 */

// WriteCSV writes a csv record
func (w *Writer) WriteCSV(msg proto.Message) (int, error) {
	return w.csvWriter.WriteRecord(msg)
}

// WriteCSVHeader writes a CSV record
func (w *Writer) WriteCSVHeader(msg proto.Message) (int, error) {
	return w.csvWriter.WriteHeader(msg)
}

/*
 *	Utils
 */

func (w *Writer) Write(msg proto.Message) error {
	if w.csv {
		// write as csv
		_, err := w.WriteCSV(msg)
		if err != nil {
			panic(err)
		}
	} else {
		// write protobuf
		err := w.WriteProto(msg)
		if err != nil {
			panic(err)
		}
	}
	return nil
}

func (w *Writer) WriteHeader(t types.Type, source string, version string, includesPayloads bool) error {
	if w.csv {
		// write as csv
		_, err := w.WriteCSVHeader(InitRecord(t))
		if err != nil {
			panic(err)
		}
	} else {
		// write protobuf
		err := w.WriteProto(NewHeader(t, source, version, includesPayloads))
		if err != nil {
			panic(err)
		}
	}
	return nil
}

type flushableWriter interface {
	Flush() error
}

func FlushWriters(writers ...flushableWriter) {
	for _, w := range writers {
		err := w.Flush()
		if err != nil {
			panic(err)
		}
	}
}

func CloseGzipWriters(writers ...*gzip.Writer) {
	for _, w := range writers {
		err := w.Flush()
		if err != nil {
			panic(err)
		}
		err = w.Close()
		if err != nil {
			panic(err)
		}
	}
}

func (w *Writer) Close() (name string, size int64) {
	if w.compress {
		CloseGzipWriters(w.gWriter)
	}
	if w.buffer {
		FlushWriters(w.bWriter)
	}
	return CloseFile(w.out, w.file, w.Name)
}

func (w *Writer) GetChan() <-chan []byte {
	return w.cWriter.Chan()
}
