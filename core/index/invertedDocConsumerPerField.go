package index

import (
	ta "github.com/balzaczyy/golucene/core/analysis/tokenattributes"
	"github.com/balzaczyy/golucene/core/index/model"
	"github.com/balzaczyy/golucene/core/util"
)

// index/InvertedDocConsumerPerField.java

type InvertedDocConsumerPerField interface {
	// Called once per field, and is given all IndexableField
	// occurrences for this field in the document. Return true if you
	// wish to see inverted tokens for these fields:
	start([]model.IndexableField, int) (bool, error)
	// Called before a field instance is being processed
	startField(model.IndexableField)
	// Called once per inverted token
	add() error
	// Called once per field per document, after all IndexableFields
	// are inverted
	finish() error
	// Called on hitting an aborting error
	abort()
}

const HASH_INIT_SIZE = 4

type TermsHashPerField struct {
	consumer TermsHashConsumerPerField

	termsHash *TermsHash

	nextPerField *TermsHashPerField
	docState     *docState
	fieldState   *FieldInvertState
	termAtt      ta.TermToBytesRefAttribute
	termBytesRef *util.BytesRef

	// Copied from our perThread
	intPool      *util.IntBlockPool
	bytePool     *util.ByteBlockPool
	termBytePool *util.ByteBlockPool

	streamCount   int
	numPostingInt int

	fieldInfo *model.FieldInfo

	bytesHash *util.BytesRefHash

	postingsArray *ParallelPostingsArray
	bytesUsed     util.Counter

	doCall, doNextCall bool

	intUptos     []int
	intUptoStart int
}

func newTermsHashPerField(docInverterPerField *DocInverterPerField,
	termsHash *TermsHash, nextTermsHash *TermsHash,
	fieldInfo *model.FieldInfo) *TermsHashPerField {

	ans := &TermsHashPerField{
		intPool:      termsHash.intPool,
		bytePool:     termsHash.bytePool,
		termBytePool: termsHash.termBytePool,
		docState:     termsHash.docState,
		termsHash:    termsHash,
		bytesUsed:    termsHash.bytesUsed,
		fieldState:   docInverterPerField.fieldState,
		fieldInfo:    fieldInfo,
	}
	ans.consumer = termsHash.consumer.addField(ans, fieldInfo)
	byteStarts := newPostingsBytesStartArray(ans, termsHash.bytesUsed)
	ans.bytesHash = util.NewBytesRefHash(termsHash.termBytePool, HASH_INIT_SIZE, byteStarts)
	ans.streamCount = ans.consumer.streamCount()
	ans.numPostingInt = 2 * ans.streamCount
	if nextTermsHash != nil {
		ans.nextPerField = nextTermsHash.addField(docInverterPerField, fieldInfo).(*TermsHashPerField)
	}
	return ans
}

func (h *TermsHashPerField) shrinkHash(targetSize int) {
	// Fully free the bytesHash on each flush but keep the pool
	// untouched. bytesHash.clear will clear the BytesStartArray and
	// in turn the ParallelPostingsArray too
	h.bytesHash.Clear(false)
}

func (h *TermsHashPerField) reset() {
	h.bytesHash.Clear(false)
	if h.nextPerField != nil {
		h.nextPerField.reset()
	}
}

func (h *TermsHashPerField) abort() {
	h.reset()
	if h.nextPerField != nil {
		h.nextPerField.abort()
	}
}

func (h *TermsHashPerField) initReader(reader *ByteSliceReader, termId, stream int) {
	assert(stream < h.streamCount)
	intStart := h.postingsArray.intStarts[termId]
	ints := h.intPool.Buffers[intStart>>util.INT_BLOCK_SHIFT]
	upto := intStart & util.INT_BLOCK_MASK
	reader.init(h.bytePool,
		h.postingsArray.byteStarts[termId]+stream*util.FIRST_LEVEL_SIZE,
		ints[upto+stream])
}

/* Collapse the hash table & sort in-place. */
func (h *TermsHashPerField) sortPostings(termComp func(a, b []byte) bool) []int {
	return h.bytesHash.Sort(termComp)
}

func (h *TermsHashPerField) startField(f model.IndexableField) {
	h.termAtt = h.fieldState.attributeSource.Get("TermToBytesRefAttribute").(ta.TermToBytesRefAttribute)
	h.termBytesRef = h.termAtt.BytesRef()
	assert(h.termBytesRef != nil)
	h.consumer.startField(f)
	if h.nextPerField != nil {
		h.nextPerField.startField(f)
	}
}

func (h *TermsHashPerField) start(fields []model.IndexableField, count int) (bool, error) {
	var err error
	h.doCall, err = h.consumer.start(fields, count)
	if err != nil {
		return false, err
	}
	h.bytesHash.Reinit()
	if h.nextPerField != nil {
		h.doNextCall, err = h.nextPerField.start(fields, count)
		if err != nil {
			return false, err
		}
	}
	return h.doCall || h.doNextCall, nil
}

/*
Secondary entry point (for 2nd & subsequent TermsHash), because token
text has already be "interned" into textStart, so we hash by textStart
*/
func (h *TermsHashPerField) addFrom(textStart int) error {
	panic("not implemented yet")
}

/* Primary entry point (for first TermsHash) */
func (h *TermsHashPerField) add() error {
	// We are first in the chain so we must "intern" the term text into
	// textStart address. Get the text & hash of this term.
	termId, ok := h.bytesHash.Add(h.termBytesRef.Value, h.termAtt.FillBytesRef())
	if !ok {
		// Not enough room in current block. Just skip this term, to
		// remain as robust as ossible during indexing. A TokenFilter can
		// be inserted into the analyzer chain if other behavior is
		// wanted (pruning the term to a prefix, returning an error, etc).
		panic("not implemented yet")
		return nil
	}

	if termId >= 0 { // new posting
		h.bytesHash.ByteStart(termId)
		// init stream slices
		if h.numPostingInt+h.intPool.IntUpto > util.INT_BLOCK_SIZE {
			h.intPool.NextBuffer()
		}

		if util.BYTE_BLOCK_SIZE-h.bytePool.ByteUpto < h.numPostingInt*util.FIRST_LEVEL_SIZE {
			panic("not implemented yet")
		}

		h.intUptos = h.intPool.Buffer
		h.intUptoStart = h.intPool.IntUpto
		h.intPool.IntUpto += h.streamCount

		h.postingsArray.intStarts[termId] = h.intUptoStart + h.intPool.IntOffset

		for i := 0; i < h.streamCount; i++ {
			upto := h.bytePool.NewSlice(util.FIRST_LEVEL_SIZE)
			h.intUptos[h.intUptoStart+i] = upto + h.bytePool.ByteOffset
		}
		h.postingsArray.byteStarts[termId] = h.intUptos[h.intUptoStart]

		err := h.consumer.newTerm(termId)
		if err != nil {
			return err
		}
	} else {
		panic("not implemented yet")
	}

	if h.doNextCall {
		return h.nextPerField.addFrom(h.postingsArray.textStarts[termId])
	}
	return nil
}

func (h *TermsHashPerField) writeByte(stream int, b byte) {
	upto := h.intUptos[h.intUptoStart+stream]
	bytes := h.bytePool.Buffers[upto>>util.BYTE_BLOCK_SHIFT]
	assert(bytes != nil)
	offset := upto & util.BYTE_BLOCK_MASK
	if bytes[offset] != 0 {
		// end of slice; allocate a new one
		panic("not implemented yet")
	}
	bytes[offset] = b
	h.intUptos[h.intUptoStart+stream]++
}

func (h *TermsHashPerField) writeVInt(stream, i int) {
	assert(stream < h.streamCount)
	for (i & ^0x7F) != 0 {
		h.writeByte(stream, byte((i&0x7F)|0x80))
	}
	h.writeByte(stream, byte(i))
}

func (h *TermsHashPerField) finish() error {
	err := h.consumer.finish()
	if err == nil && h.nextPerField != nil {
		err = h.nextPerField.finish()
	}
	return err
}

type PostingsBytesStartArray struct {
	perField  *TermsHashPerField
	bytesUsed util.Counter
}

func newPostingsBytesStartArray(perField *TermsHashPerField,
	bytesUsed util.Counter) *PostingsBytesStartArray {
	return &PostingsBytesStartArray{perField, bytesUsed}
}

func (ss *PostingsBytesStartArray) Init() []int {
	if ss.perField.postingsArray == nil {
		arr := ss.perField.consumer.createPostingsArray(2)
		ss.bytesUsed.AddAndGet(int64(arr.size * arr.bytesPerPosting()))
		ss.perField.postingsArray = arr
	}
	return ss.perField.postingsArray.textStarts
}

func (ss *PostingsBytesStartArray) Grow() []int {
	panic("not implemented yet")
}

func (ss *PostingsBytesStartArray) Clear() []int {
	if ss.perField.postingsArray != nil {
		ss.bytesUsed.AddAndGet(-int64(ss.perField.postingsArray.size * ss.perField.postingsArray.bytesPerPosting()))
		ss.perField.postingsArray = nil
	}
	return nil
}

func (ss *PostingsBytesStartArray) BytesUsed() util.Counter {
	return ss.bytesUsed
}

// index/ParallelPostingsArray.java

const BYTES_PER_POSTING = 3 * util.NUM_BYTES_INT

type PostingsArray interface {
	bytesPerPosting() int
	newInstance(size int) PostingsArray
	copyTo(toArray PostingsArray, numToCopy int)
}

type ParallelPostingsArray struct {
	PostingsArray
	size       int
	textStarts []int
	intStarts  []int
	byteStarts []int
}

func newParallelPostingsArray(spi PostingsArray, size int) *ParallelPostingsArray {
	return &ParallelPostingsArray{
		PostingsArray: spi,
		size:          size,
		textStarts:    make([]int, size),
		intStarts:     make([]int, size),
		byteStarts:    make([]int, size),
	}
}

func (arr *ParallelPostingsArray) grow() *ParallelPostingsArray {
	panic("not implemented yet")
}
