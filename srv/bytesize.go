package srv

import (
	"github.com/docker/go-units"
)

// ByteSize used to pass byte sizes to a go-flags CLI
type ByteSize struct {
	Value uint64
}

// NewByteSize creates a new ByteSize instance with the given value.
func NewByteSize(value uint64) *ByteSize {
	return &ByteSize{Value: value}
}

// String method for a bytesize (pflag value and stringer interface)
func (b *ByteSize) String() string {
	return units.HumanSize(float64(b.Value))
}

// Set the value of this bytesize (pflag value interfaces)
func (b *ByteSize) Set(value string) error {
	sz, err := units.FromHumanSize(value)
	if err != nil {
		return err
	}
	b.Value = uint64(sz)
	return nil
}

// Type returns the type of the pflag value (pflag value interface)
func (b *ByteSize) Type() string {
	return "byte-size"
}

// Get returns the underlying uint64 value of ByteSize.
func (b *ByteSize) Get() uint64 {
	return b.Value
}

