package bloomfilter

import (
	"fmt"
	"github.com/GrappigPanda/Olivia/lru"
	"github.com/mtchavez/jenkins"
	"github.com/spaolacci/murmur3"
	"hash/fnv"
	"math"
)

type BloomFilter interface {
	AddKey(key []byte) (bool, []uint)
	HasKey(key []byte) (bool, []uint)
	Serialize() string
	GetMaxSize() uint
	GetStorage() Bitset
	Compare(interface{}) bool
	HashKey([]byte) []uint
}

type SimpleBloomFilter struct {
	// The maximum size for the bloom filter
	maxSize uint
	// Total number of hashing functions
	HashFunctions uint
	filter        Bitset
	HashCache     *lru.LRUCacheInt32Array
}

// New Returns a pointer to a newly allocated `SimpleBloomFilter` object
func NewSimpleBF(maxSize uint, hashFuns uint) *SimpleBloomFilter {
	return &SimpleBloomFilter{
		maxSize,
		hashFuns,
		NewWFBitset(maxSize),
		lru.NewInt32Array(int((float64(maxSize) * float64(0.1)))),
	}
}

// NewByFailRate allows generation of a bloom filter with a pre-conceived
// amount of items and a false-positive failure rate. We calculate our bloom
// filter bounds and generate the new bloom filter this way.
func NewByFailRate(items uint, probability float64) *SimpleBloomFilter {
	m, k := estimateBounds(items, probability)
	return NewSimpleBF(m, k)
}

// GetMaxSize returns the max size. Just an ugly getter.
func (bf *SimpleBloomFilter) GetMaxSize() uint {
	return bf.maxSize
}

// AddKey Adds a new key to the bloom filter
func (bf *SimpleBloomFilter) AddKey(key []byte) (bool, []uint) {
	hasKey, hashIndexes := bf.HasKey(key)
	if !hasKey {
		hashIndexes = bf.HashKey(key)
	}

	for _, index := range hashIndexes {
		bf.filter.Add(index)
	}

	return true, hashIndexes
}

// HasKey verifies if a key is or isn't in the bloom filter.
func (bf *SimpleBloomFilter) HasKey(key []byte) (bool, []uint) {
	hashIndexes := bf.HashKey(key)

	for _, element := range hashIndexes {
		if bf.filter.Contains(element) {
			continue
		} else {
			return false, nil
		}
	}

	return true, hashIndexes
}

// ConvertToString handles conversion of a bloom filter to a string. Moreover,
// it enforces RLE encoding, so that fewer bytes are transferred per request.
func (bf *SimpleBloomFilter) Serialize() string {
	return Encode(bf.filter.ToString())
}

// ConvertStringToBF Decodes the RLE'd bloom filter and then converts it to
// an actual bloom filter in-memory.
func Deserialize(inputString string, maxSize uint) (*SimpleBloomFilter, error) {
	bf := NewByFailRate(maxSize, 0.01)

	sz := fmt.Sprintf("\"%s=\"", Decode(inputString))
	bf.filter.FromString(sz)

	return bf, nil
}

// estimateBounds Generates the bounds for total hash function calls and for
// the total bloom filter size
func estimateBounds(items uint, probability float64) (uint, uint) {
	// https://en.wikipedia.org/wiki/Bloom_filter#Counting_filters
	// See "Optimal number of hash functions section"
	n := items
	m := (-1 * float64(n) * math.Log(probability)) / (math.Pow(math.Log(2), 2))
	k := uint((m / float64(n)) * math.Log(2))

	return uint(m), k
}

// calculateHash Takes in a string and calculates the 64bit hash value.
func calculateHash(key []byte, offSet int) uint {
	switch offSet {
	// By Default/for offset 1 we'll just use FNV
	default:
		hasher := fnv.New32()
		hasher.Write(key)
		return uint(hasher.Sum32())
	case 1:
		hasher := murmur3.New32()
		hasher.Write(key)
		return uint(hasher.Sum32())
	case 2:
		hasher := jenkins.New()
		hasher.Write(key)
		return uint(hasher.Sum32())
	}

	return 0
}

// HashKey Takes a string in as an argument and hashes it several times to
// create usable indexes for the bloom filter.
func (bf *SimpleBloomFilter) HashKey(key []byte) []uint {
	hashes := make([]uint, bf.HashFunctions)

	for index := range hashes {
		hashes[index] = calculateHash(key, index) % uint(bf.GetMaxSize())
	}

	return hashes
}

// GetStorage handles returning the underlying bloomfilter bitset.
func (bf *SimpleBloomFilter) GetStorage() Bitset {
	return bf.filter
}

// Compare returns if the two bloomfilters are equal
func (bf *SimpleBloomFilter) Compare(remote interface{}) bool {
	return bf.filter.Compare(remote.(*SimpleBloomFilter).GetStorage())
}
