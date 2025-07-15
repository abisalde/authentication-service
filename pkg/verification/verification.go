package verification

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

func GenerateVerificationCode() string {
	var n uint16
	_ = binary.Read(rand.Reader, binary.LittleEndian, &n)
	code := int(n % 10000)
	return fmt.Sprintf("%04d", code)
}
