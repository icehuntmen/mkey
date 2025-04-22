// Package mkey provides an enhanced Snowflake ID generator with additional features
package mkey

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"sync"
	"time"
)

const (
	// DefaultEpoch is set to the Manticora Syndicate epoch (Nov 12 2024 17:00:00 UTC)
	DefaultEpoch int64 = 1731430800000

	// DefaultNodeBits is the default number of bits to use for Node
	DefaultNodeBits uint8 = 10

	// DefaultStepBits is the default number of bits to use for Step
	DefaultStepBits uint8 = 12

	// MaxNodeBits is the maximum allowed bits for Node
	MaxNodeBits uint8 = 16

	// MaxStepBits is the maximum allowed bits for Step
	MaxStepBits uint8 = 16
)

// Custom encoding maps
const (
	encodeBase32Map = "7w3x5h9k2m4p6q8r1sdyfgjtnvzbcaeu"
	encodeBase58Map = "123456789abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ"
	encodeBase64Map = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
)

var (
	decodeBase32Map [256]byte
	decodeBase58Map [256]byte
	decodeBase64Map [256]byte
)

// Initialize decoding maps
func init() {
	initDecodeMap(encodeBase32Map, &decodeBase32Map)
	initDecodeMap(encodeBase58Map, &decodeBase58Map)
	initDecodeMap(encodeBase64Map, &decodeBase64Map)
}

func initDecodeMap(encodeMap string, decodeMap *[256]byte) {
	for i := 0; i < len(decodeMap); i++ {
		decodeMap[i] = 0xFF
	}
	for i := 0; i < len(encodeMap); i++ {
		decodeMap[encodeMap[i]] = byte(i)
	}
}

// Config holds the configuration for Snowflake generator
type Config struct {
	Epoch    int64
	NodeBits uint8
	StepBits uint8
	Node     int64
}

// Node represents a snowflake generator node
type Node struct {
	mu    sync.Mutex
	epoch time.Time
	time  int64
	node  int64
	step  int64

	// Precomputed values
	nodeMax   int64
	nodeMask  int64
	stepMask  int64
	timeShift uint8
	nodeShift uint8
}

// ID is a custom type for snowflake ID
type ID int64

// NewConfig creates a new Config with default values
func NewConfig() *Config {
	return &Config{
		Epoch:    DefaultEpoch,
		NodeBits: DefaultNodeBits,
		StepBits: DefaultStepBits,
	}
}

// NewNode creates a new snowflake node with default config
func NewNode(node int64) (*Node, error) {
	return NewNodeWithConfig(&Config{
		Epoch:    DefaultEpoch,
		NodeBits: DefaultNodeBits,
		StepBits: DefaultStepBits,
		Node:     node,
	})
}

// NewNodeWithConfig creates a new snowflake node with custom configuration
func NewNodeWithConfig(cfg *Config) (*Node, error) {
	// Validate configuration
	if cfg.NodeBits > MaxNodeBits {
		return nil, fmt.Errorf("NodeBits must be <= %d", MaxNodeBits)
	}
	if cfg.StepBits > MaxStepBits {
		return nil, fmt.Errorf("StepBits must be <= %d", MaxStepBits)
	}
	if cfg.NodeBits+cfg.StepBits > 22 {
		return nil, errors.New("NodeBits + StepBits must be <= 22")
	}

	nodeMax := -1 ^ (-1 << cfg.NodeBits)
	if cfg.Node < 0 || cfg.Node > int64(nodeMax) {
		return nil, fmt.Errorf("Node must be between 0 and %d", nodeMax)
	}

	n := &Node{
		node:      cfg.Node,
		nodeMax:   int64(nodeMax),
		nodeMask:  int64(nodeMax) << cfg.StepBits,
		stepMask:  -1 ^ (-1 << cfg.StepBits),
		timeShift: cfg.NodeBits + cfg.StepBits,
		nodeShift: cfg.StepBits,
	}

	// Setup epoch
	curTime := time.Now()
	n.epoch = curTime.Add(time.Unix(cfg.Epoch/1000, (cfg.Epoch%1000)*1000000).Sub(curTime))

	return n, nil
}

// Generate creates and returns a unique snowflake ID
func (n *Node) Generate() ID {
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Since(n.epoch).Nanoseconds() / 1000000

	if now == n.time {
		n.step = (n.step + 1) & n.stepMask

		if n.step == 0 {
			for now <= n.time {
				now = time.Since(n.epoch).Nanoseconds() / 1000000
			}
		}
	} else {
		n.step = 0
	}

	n.time = now

	return ID((now)<<n.timeShift |
		(n.node << n.nodeShift) |
		(n.step))
}

// GenerateBatch generates multiple IDs at once (more efficient for bulk operations)
func (n *Node) GenerateBatch(count int) ([]ID, error) {
	if count <= 0 {
		return nil, errors.New("count must be positive")
	}
	if count > int(n.stepMask) {
		return nil, fmt.Errorf("count must be <= %d", n.stepMask)
	}

	ids := make([]ID, count)
	n.mu.Lock()
	defer n.mu.Unlock()

	now := time.Since(n.epoch).Nanoseconds() / 1000000

	if now == n.time {
		// If we're at the same time, we need to make sure we have enough step space
		if n.step+int64(count) > n.stepMask {
			// Not enough space in current millisecond, wait for next
			for now <= n.time {
				now = time.Since(n.epoch).Nanoseconds() / 1000000
			}
			n.step = 0
		}
	} else {
		n.step = 0
	}

	n.time = now

	for i := 0; i < count; i++ {
		ids[i] = ID((now)<<n.timeShift |
			(n.node << n.nodeShift) |
			(n.step))
		n.step++
	}

	return ids, nil
}

// RandomNodeID generates a random node ID within the allowed range
func (n *Node) RandomNodeID() (int64, error) {
	max := big.NewInt(n.nodeMax + 1)
	randNum, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}
	return randNum.Int64(), nil
}

// Int64 returns the int64 value of the ID
func (f ID) Int64() int64 {
	return int64(f)
}

// Time returns the timestamp component of the ID
func (f ID) Time(node *Node) int64 {
	return (int64(f) >> node.timeShift) + node.epoch.UnixNano()/1000000
}

// NodeID returns the node component of the ID
func (f ID) NodeID(node *Node) int64 {
	return int64(f) & node.nodeMask >> node.nodeShift
}

// Step returns the step component of the ID
func (f ID) Step(node *Node) int64 {
	return int64(f) & node.stepMask
}

// String returns a decimal string representation of the ID
func (f ID) String() string {
	return strconv.FormatInt(int64(f), 10)
}

// Base2 returns a base2 (binary) string representation
func (f ID) Base2() string {
	return strconv.FormatInt(int64(f), 2)
}

// Base32 returns a base32 encoded string using custom encoding
func (f ID) Base32() string {
	if f == 0 {
		return string(encodeBase32Map[0])
	}

	b := make([]byte, 0, 12)
	for f > 0 {
		b = append(b, encodeBase32Map[f%32])
		f /= 32
	}

	// Reverse the slice
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// Base58 returns a base58 encoded string
func (f ID) Base58() string {
	if f == 0 {
		return string(encodeBase58Map[0])
	}

	b := make([]byte, 0, 11)
	for f > 0 {
		b = append(b, encodeBase58Map[f%58])
		f /= 58
	}

	// Reverse the slice
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}

	return string(b)
}

// Base64 returns a URL-safe base64 encoded string
func (f ID) Base64() string {
	data := make([]byte, 8)
	binary.BigEndian.PutUint64(data, uint64(f))

	// Trim leading zeros
	i := 0
	for ; i < len(data) && data[i] == 0; i++ {
	}

	return base64.RawURLEncoding.EncodeToString(data[i:])
}

// Bytes returns the ID as a byte slice (big endian)
func (f ID) Bytes() []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(f))
	return b
}

// Timestamp returns the time.Time representation of the timestamp component
func (f ID) Timestamp(node *Node) time.Time {
	ms := (int64(f) >> node.timeShift) + node.epoch.UnixNano()/1000000
	return time.Unix(ms/1000, (ms%1000)*1000000)
}

// MarshalJSON implements json.Marshaler
func (f ID) MarshalJSON() ([]byte, error) {
	return []byte(f.String()), nil
}

// UnmarshalJSON implements json.Unmarshaler
func (f *ID) UnmarshalJSON(data []byte) error {
	id, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return err
	}
	*f = ID(id)
	return nil
}

// Parse functions for different encodings
func ParseBase32(b []byte) (ID, error) {
	var id int64
	for _, c := range b {
		if decodeBase32Map[c] == 0xFF {
			return 0, errors.New("invalid base32 character")
		}
		id = id*32 + int64(decodeBase32Map[c])
	}
	return ID(id), nil
}

func ParseBase58(b []byte) (ID, error) {
	var id int64
	for _, c := range b {
		if decodeBase58Map[c] == 0xFF {
			return 0, errors.New("invalid base58 character")
		}
		id = id*58 + int64(decodeBase58Map[c])
	}
	return ID(id), nil
}

func ParseBase64(b []byte) (ID, error) {
	data, err := base64.RawURLEncoding.DecodeString(string(b))
	if err != nil {
		return 0, err
	}

	var id uint64
	for _, b := range data {
		id = (id << 8) | uint64(b)
	}

	return ID(id), nil
}
