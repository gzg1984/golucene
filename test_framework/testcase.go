package test_framework

import (
	"fmt"
	"github.com/balzaczyy/golucene/core/analysis"
	"github.com/balzaczyy/golucene/core/index"
	"github.com/balzaczyy/golucene/core/search"
	"github.com/balzaczyy/golucene/core/store"
	"github.com/balzaczyy/golucene/core/util"
	ti "github.com/balzaczyy/golucene/test_framework/index"
	ts "github.com/balzaczyy/golucene/test_framework/search"
	. "github.com/balzaczyy/golucene/test_framework/util"
	. "github.com/balzaczyy/gounit"
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"reflect"
)

// --------------------------------------------------------------------
// Test groups, system properties and other annotations modifying tests
// --------------------------------------------------------------------

// -----------------------------------------------------------------
// Truly immutable fields and constants, initialized once and valid
// for all suites ever since.
// -----------------------------------------------------------------

// Use this constant then creating Analyzers and any other version-dependent
// stuff. NOTE: Change this when developmenet starts for new Lucene version:
const TEST_VERSION_CURRENT = util.VERSION_45

// Throttling
var TEST_THROTTLING = either(TEST_NIGHTLY, THROTTLING_SOMETIMES, THROTTLING_NEVER).(Throttling)

func either(flag bool, value, orValue interface{}) interface{} {
	if flag {
		return value
	}
	return orValue
}

// L300

// -----------------------------------------------------------------
// Class level (suite) rules.
// -----------------------------------------------------------------

// Class environment setup rule.
var ClassEnvRule = &TestRuleSetupAndRestoreClassEnv{}

// -----------------------------------------------------------------
// Test facilities and facades for subclasses.
// -----------------------------------------------------------------

// Create a new index writer config with random defaults
func NewIndexWriterConfig(v util.Version, a analysis.Analyzer) *index.IndexWriterConfig {
	return newRandomIndexWriteConfig(Random(), v, a)
}

// Create a new index write config with random defaults using the specified random
func newRandomIndexWriteConfig(r *rand.Rand, v util.Version, a analysis.Analyzer) *index.IndexWriterConfig {
	c := index.NewIndexWriterConfig(v, a)
	c.SetSimilarity(ClassEnvRule.similarity)
	if VERBOSE {
		panic("not implemented yet")
	}

	if r.Intn(2) == 0 {
		c.SetMergeScheduler(index.NewSerialMergeScheduler())
	} else if Rarely(r) {
		maxRoutineCount := NextInt(Random(), 1, 4)
		maxMergeCount := NextInt(Random(), maxRoutineCount, maxRoutineCount+4)
		cms := index.NewConcurrentMergeScheduler()
		cms.SetMaxMergesAndRoutines(maxMergeCount, maxRoutineCount)
		c.SetMergeScheduler(cms)
	}
	if r.Intn(2) == 0 {
		if Rarely(r) {
			// crazy value
			c.SetMaxBufferedDocs(NextInt(r, 2, 15))
		} else {
			// reasonable value
			c.SetMaxBufferedDocs(NextInt(r, 16, 1000))
		}
	}
	// Go doesn't need thread-affinity state.
	// if r.Intn(2) == 0 {
	// 	maxNumRoutineState := either(Rarely(r),
	// 		NextInt(r, 5, 20), // crazy value
	// 		NextInt(r, 1, 4))  // reasonable value

	// 	if Rarely(r) {
	// 		// reandom thread pool
	// 		c.SetIndexerThreadPool(newRandomDocumentsWriterPerThreadPool(maxNumRoutineState, r))
	// 	} else {
	// 		// random thread pool
	// 		c.SetMaxThreadStates(maxNumRoutineState)
	// 	}
	// }

	c.SetMergePolicy(newMergePolicy(r))

	if Rarely(r) {
		c.SetMergedSegmentWarmer(index.NewSimpleMergedSegmentWarmer(c.InfoStream()))
	}
	c.SetUseCompoundFile(r.Intn(2) == 0)
	c.SetReaderPooling(r.Intn(2) == 0)
	c.SetReaderTermsIndexDivisor(NextInt(r, 1, 4))
	return c
}

func newMergePolicy(r *rand.Rand) index.MergePolicy {
	if Rarely(r) {
		return ti.NewMockRandomMergePolicy(r)
	} else if r.Intn(2) == 0 {
		return newTieredMergePolicy(r)
	} else if r.Intn(5) == 0 {
		return newAlcoholicMergePolicy(r /*, ClassEnvRule.timeZone*/)
	} else {
		return newLogMergePolicy(r)
	}
}

// L883
func newTieredMergePolicy(r *rand.Rand) *index.TieredMergePolicy {
	tmp := index.NewTieredMergePolicy()
	if Rarely(r) {
		tmp.SetMaxMergeAtOnce(NextInt(r, 2, 9))
		tmp.SetMaxMergeAtOnceExplicit(NextInt(r, 2, 9))
	} else {
		tmp.SetMaxMergeAtOnce(NextInt(r, 10, 50))
		tmp.SetMaxMergeAtOnceExplicit(NextInt(r, 10, 50))
	}
	if Rarely(r) {
		tmp.SetMaxMergedSegmentMB(0.2 + r.Float64()*100)
	} else {
		tmp.SetMaxMergedSegmentMB(r.Float64() * 100)
	}
	tmp.SetFloorSegmentMB(0.2 + r.Float64()*2)
	tmp.SetForceMergeDeletesPctAllowed(0 + r.Float64()*30)
	if Rarely(r) {
		tmp.SetSegmentsPerTier(float64(NextInt(r, 2, 20)))
	} else {
		tmp.SetSegmentsPerTier(float64(NextInt(r, 10, 50)))
	}
	configureRandom(r, tmp)
	tmp.SetReclaimDeletesWeight(r.Float64() * 4)
	return tmp
}

func newAlcoholicMergePolicy(r *rand.Rand /*, tz TimeZone*/) *ti.AlcoholicMergePolicy {
	return newAlcoholicMergePolicy(rand.New(rand.NewSource(r.Int63())))
}

func newLogMergePolicy(r *rand.Rand) *index.LogMergePolicy {
	var logmp *index.LogMergePolicy
	if r.Intn(2) == 0 {
		logmp = index.NewLogDocMergePolicy()
	} else {
		logmp = index.NewLogByteSizeMergePolicy()
	}
	if Rarely(r) {
		logmp.SetMergeFactor(NextInt(r, 2, 9))
	} else {
		logmp.SetMergeFactor(NextInt(r, 10, 50))
	}
	configureRandom(r, logmp)
	return logmp
}

func configureRandom(r *rand.Rand, mergePolicy index.MergePolicy) {
	if r.Intn(2) == 0 {
		mergePolicy.SetNoCFSRatio(0.1 + r.Float64())
	} else if r.Intn(2) == 0 {
		mergePolicy.SetNoCFSRatio(1.0)
	} else {
		mergePolicy.SetNoCFSRatio(0)
	}

	if Rarely(r) {
		mergePolicy.SetMaxCFSSegmentSizeMB(0.2 + r.Float64()*2)
	} else {
		mergePolicy.SetMaxCFSSegmentSizeMB(math.Inf(1))
	}
}

/*
Returns a new Direcotry instance. Use this when the test does not care about
the specific Directory implementation (most tests).

The Directory is wrapped with BaseDirectoryWrapper. This menas usually it
will be picky, such as ensuring that you properly close it and all open files
in your test. It will emulate some features of Windows, such as not allowing
open files ot be overwritten.
*/
func NewDirectory() BaseDirectoryWrapper {
	return newDirectoryWithSeed(Random())
}

// Returns a new Directory instance, using the specified random.
// See NewDirecotry() for more information
func newDirectoryWithSeed(r *rand.Rand) BaseDirectoryWrapper {
	return wrapDirectory(r, newDirectoryImpl(r, TEST_DIRECTORY), Rarely(r))
}

func wrapDirectory(random *rand.Rand, directory store.Directory, bare bool) BaseDirectoryWrapper {
	if Rarely(random) {
		directory = store.NewNRTCachingDirectory(directory, random.Float64(), random.Float64())
	}

	if Rarely(random) {
		maxMBPerSec := 10 + 5*(random.Float64()-0.5)
		if VERBOSE {
			log.Printf("LuceneTestCase: will rate limit output IndexOutput to %v MB/sec", maxMBPerSec)
		}
		rateLimitedDirectoryWrapper := store.NewRateLimitedDirectoryWrapper(directory)
		switch random.Intn(10) {
		case 3: // sometimes rate limit on flush
			rateLimitedDirectoryWrapper.SetMaxWriteMBPerSec(maxMBPerSec, store.IO_CONTEXT_TYPE_FLUSH)
		case 2: // sometimes rate limit flush & merge
			rateLimitedDirectoryWrapper.SetMaxWriteMBPerSec(maxMBPerSec, store.IO_CONTEXT_TYPE_FLUSH)
			rateLimitedDirectoryWrapper.SetMaxWriteMBPerSec(maxMBPerSec, store.IO_CONTEXT_TYPE_MERGE)
		default:
			rateLimitedDirectoryWrapper.SetMaxWriteMBPerSec(maxMBPerSec, store.IO_CONTEXT_TYPE_MERGE)
		}
		directory = rateLimitedDirectoryWrapper
	}

	if bare {
		base := NewBaseDirectoryWrapper(directory)
		CloseAfterSuite(NewCloseableDirectory(base, SuiteFailureMarker))
		return base
	} else {
		mock := NewMockDirectoryWrapper(random, directory)

		mock.SetThrottling(TEST_THROTTLING)
		CloseAfterSuite(NewCloseableDirectory(mock, SuiteFailureMarker))
		return mock
	}
}

// L1064
func NewTextField(name, value string, stored bool) *index.Field {
	flag := index.TEXT_FIELD_TYPE_STORED
	if !stored {
		flag = index.TEXT_FIELD_TYPE_NOT_STORED
	}
	return NewField(Random(), name, value, flag)
}

func NewField(r *rand.Rand, name, value string, typ *index.FieldType) *index.Field {
	panic("not implemented yet")
}

// Ian: Different from Lucene's default random class initializer, I have to
// explicitly initialize different directory randomly.
func newDirectoryImpl(random *rand.Rand, clazzName string) store.Directory {
	if clazzName == "random" {
		if Rarely(random) {
			switch random.Intn(1) {
			case 0:
				clazzName = "SimpleFSDirectory"
			}
		} else {
			clazzName = "RAMDirectory"
		}
	}
	if clazzName == "RAMDirectory" {
		return store.NewRAMDirectory()
	} else {
		path := TempDir("index")
		if err := os.MkdirAll(path, os.ModeTemporary); err != nil {
			panic(err)
		}
		switch clazzName {
		case "SimpleFSDirectory":
			d, err := store.NewSimpleFSDirectory(path)
			if err != nil {
				panic(err)
			}
			return d
		}
		panic(fmt.Sprintf("not supported yet: %v", clazzName))
	}
}

// L1305
// Create a new searcher over the reader. This searcher might randomly use threads
func NewSearcher(r index.IndexReader) *search.IndexSearcher {
	panic("not implemented yet")
}

// util/TestRuleSetupAndRestoreClassEnv.java

var suppressedCodecs string

func SuppressCodecs(name string) {
	suppressedCodecs = name
}

type ThreadNameFixingPrintStreamInfoStream struct {
	*util.PrintStreamInfoStream
}

func newThreadNameFixingPrintStreamInfoStream(w io.Writer) *ThreadNameFixingPrintStreamInfoStream {
	panic("not implemented yet")
}

func (is *ThreadNameFixingPrintStreamInfoStream) Message(component, message string) {
	panic("not implemented yet")
}

func (is *ThreadNameFixingPrintStreamInfoStream) Clone() util.InfoStream {
	clone := *is
	return &clone
}

// Setup and restore suite-level environment (fine grained junk that
// doesn't fit anywhere else)
type TestRuleSetupAndRestoreClassEnv struct {
	savedCodec      index.Codec
	savedInfoStream util.InfoStream

	similarity search.Similarity
	codec      index.Codec

	avoidCodecs map[string]bool
}

func (rule *TestRuleSetupAndRestoreClassEnv) Before() error {
	// if verbose: print some debugging stuff about which codecs are loaded.
	if VERBOSE {
		for _, codec := range index.AvailableCodecs() {
			log.Printf("Loaded codec: '%v': %v", codec, reflect.TypeOf(index.LoadCodec(codec)).Name())
		}

		panic("not implemented yet")
	}

	rule.savedInfoStream = util.DefaultInfoStream()
	random := Random()
	if INFOSTREAM {
		util.SetDefaultInfoStream(newThreadNameFixingPrintStreamInfoStream(os.Stdout))
	} else if random.Intn(2) == 0 {
		util.SetDefaultInfoStream(NewNullInfoStream())
	}

	rule.avoidCodecs = make(map[string]bool)
	if suppressedCodecs != "" {
		rule.avoidCodecs[suppressedCodecs] = true
	}

	PREFLEX_IMPERSONATION_IS_ACTIVE = false
	rule.savedCodec = index.DefaultCodec()
	randomVal := random.Intn(10)
	if "Lucene3x" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			"random" == TEST_POSTINGSFORMAT &&
			"random" == TEST_DOCVALUESFORMAT &&
			randomVal == 3 &&
			!rule.shouldAvoidCodec("Lucene3x") { // preflex-only setup
		panic("not supported yet")
	} else if "Lucene40" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			"random" == TEST_POSTINGSFORMAT &&
			randomVal == 0 &&
			!rule.shouldAvoidCodec("Lucene40") { // 4.0 setup
		panic("not supported yet")
	} else if "Lucene41" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			"random" == TEST_POSTINGSFORMAT &&
			"random" == TEST_DOCVALUESFORMAT &&
			randomVal == 1 &&
			!rule.shouldAvoidCodec("Lucene41") {
		panic("not supported yet")
	} else if "Lucene42" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			"random" == TEST_POSTINGSFORMAT &&
			"random" == TEST_DOCVALUESFORMAT &&
			randomVal == 2 &&
			!rule.shouldAvoidCodec("Lucene42") {
		panic("not supported yet")
	} else if "random" != TEST_POSTINGSFORMAT ||
		"random" != TEST_DOCVALUESFORMAT {
		// the user wired postings or DV: this is messy
		// refactor into RandomCodec...

		panic("not supported yet")
	} else if "SimpleText" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			randomVal == 9 &&
			Rarely(random) &&
			!rule.shouldAvoidCodec("SimpleText") {
		panic("not supported yet")
	} else if "Appending" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			randomVal == 8 &&
			!rule.shouldAvoidCodec("Appending") {
		panic("not supported yet")
	} else if "CheapBastard" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			randomVal == 8 &&
			!rule.shouldAvoidCodec("CheapBastard") &&
			!rule.shouldAvoidCodec("Lucene41") {
		panic("not supported yet")
	} else if "Asserting" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			randomVal == 6 &&
			!rule.shouldAvoidCodec("Asserting") {
		panic("not implemented yet")
	} else if "Compressing" == TEST_CODEC ||
		"random" == TEST_CODEC &&
			randomVal == 5 &&
			!rule.shouldAvoidCodec("Compressing") {
		panic("not supported yet")
	} else if "random" != TEST_CODEC {
		rule.codec = index.LoadCodec(TEST_CODEC)
	} else if "random" == TEST_POSTINGSFORMAT {
		panic("not supported yet")
	} else {
		assert(false)
	}
	index.DefaultCodec = func() index.Codec { return rule.codec }

	// Initialize locale/ timezone
	// testLocale := or(os.Getenv("tests.locale"), "random")
	// testTimeZon := or(os.Getenv("tests.timezone"), "random")

	// Always pick a random one for consistency (whether tests.locale
	// was specified or not)
	// Ian: it's not supported yet
	// rule.savedLocale := DefaultLocale()
	// if "random" == testLocale {
	// 	rule.locale = randomLocale(random)
	// } else {
	// 	rule.locale = localeForName(testLocale)
	// }
	// SetDefaultLocale(rule.locale)

	// SetDefaultTimeZone() will set user.timezone to the default
	// timezone of the user's locale. So store the original property
	// value and restore it at end.
	// rule.restoreProperties["user.timezone"] = os.Getenv("user.timezone")
	// rule.savedTimeZone = DefaultTimeZone()
	// if "random" == testTimeZone {
	// 	rule.timeZone = randomTimeZone(random)
	// } else {
	// 	rule.timeZone = TimeZone(testTimeZone)
	// }
	// SetDefaultTImeZone(rule.timeZone)

	if random.Intn(2) == 0 {
		rule.similarity = search.NewDefaultSimilarity()
	} else {
		rule.similarity = ts.NewRandomSimilarityProvider(random)
	}

	// Check codec restrictions once at class level.
	err := rule.checkCodecRestrictions(rule.codec)
	if err != nil {
		log.Printf("NOTE: %v Suppressed codecs: %v", err, rule.avoidCodecs)
	}
	return err
}

func or(a, b string) string {
	if len(a) > 0 {
		return a
	}
	return b
}

// Check codec restrictions.
func (rule *TestRuleSetupAndRestoreClassEnv) checkCodecRestrictions(codec index.Codec) error {
	AssumeTrue(fmt.Sprintf("Class not allowed to use codec: %v.", codec.Name),
		rule.shouldAvoidCodec(codec.Name()))

	if _, ok := codec.(*index.RandomCodec); ok && len(rule.avoidCodecs) > 0 {
		panic("not implemented yet")
	}

	pf := codec.PostingsFormat()
	AssumeFalse(fmt.Sprintf("Class not allowed to use postings format: %v.", pf.Name()),
		rule.shouldAvoidCodec(pf.Name()))

	AssumeFalse(fmt.Sprintf("Class not allowed to use postings format: %v.", TEST_POSTINGSFORMAT),
		rule.shouldAvoidCodec(TEST_POSTINGSFORMAT))

	return nil
}

func (rule *TestRuleSetupAndRestoreClassEnv) After() error {
	panic("not implemented yet")
}

// Should a given codec be avoided for the currently executing suite?
func (rule *TestRuleSetupAndRestoreClassEnv) shouldAvoidCodec(codec string) bool {
	if len(rule.avoidCodecs) == 0 {
		return false
	}
	_, ok := rule.avoidCodecs[codec]
	return ok
}