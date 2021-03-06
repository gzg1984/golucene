package lucene71

import (
	"github.com/gzg1984/golucene/core/codec/lucene40"
	"github.com/gzg1984/golucene/core/codec/lucene41"
	"github.com/gzg1984/golucene/core/codec/lucene42"
	"github.com/gzg1984/golucene/core/codec/lucene46"
	"github.com/gzg1984/golucene/core/codec/lucene49"
	"github.com/gzg1984/golucene/core/codec/perfield"
	. "github.com/gzg1984/golucene/core/codec/spi"
)

func init() {
	RegisterCodec(newLucene71Codec())
}

/*Lucene71Codec implements the Lucene 4.10 index format,
with configuration per-field
postings and docvalues format.

If you want to reuse functionality of this codec in another codec,
extend FilterCodec (or embeds the Codec in Go style).
*/
type Lucene71Codec struct {
	*CodecImpl
}

func newLucene71Codec() *Lucene71Codec {
	return &Lucene71Codec{NewCodec("Lucene71",
		lucene41.NewLucene41StoredFieldsFormat(),
		lucene42.NewLucene42TermVectorsFormat(),
		lucene46.NewLucene46FieldInfosFormat(),
		NewLucene46SegmentInfoFormat(),
		new(lucene40.Lucene40LiveDocsFormat),
		perfield.NewPerFieldPostingsFormat(func(field string) PostingsFormat {
			return LoadPostingsFormat("Lucene41")
		}),
		perfield.NewPerFieldDocValuesFormat(func(field string) DocValuesFormat {
			panic("not implemented yet")
		}),
		new(lucene49.Lucene49NormsFormat),
	)}
}
