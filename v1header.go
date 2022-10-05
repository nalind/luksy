package lukstool

import (
	"crypto/rand"
	"errors"
	"fmt"
	"syscall"
)

type V1Header [592]uint8
type V1KeySlot [48]uint8

const (
	// Mostly verbatim from LUKS1 On-Disk Format Specification version 1.2.3
	V1Magic               = "LUKS\xba\xbe"
	v1MagicStart          = 0
	v1MagicLength         = 6
	v1VersionStart        = v1MagicStart + v1MagicLength
	v1VersionLength       = 2
	v1CipherNameStart     = v1VersionStart + v1VersionLength
	v1CipherNameLength    = 32
	v1CipherModeStart     = v1CipherNameStart + v1CipherNameLength
	v1CipherModeLength    = 32
	v1HashSpecStart       = v1CipherModeStart + v1CipherModeLength
	v1HashSpecLength      = 32
	v1PayloadOffsetStart  = v1HashSpecStart + v1HashSpecLength
	v1PayloadOffsetLength = 4
	v1KeyBytesStart       = v1PayloadOffsetStart + v1PayloadOffsetLength
	v1KeyBytesLength      = 4
	v1MKDigestStart       = v1KeyBytesStart + v1KeyBytesLength
	v1MKDigestLength      = V1DigestSize
	v1MKDigestSaltStart   = v1MKDigestStart + v1MKDigestLength
	v1MKDigestSaltLength  = V1SaltSize
	v1MKDigestIterStart   = v1MKDigestSaltStart + v1MKDigestSaltLength
	v1MKDigestIterLength  = 4
	v1UUIDStart           = v1MKDigestIterStart + v1MKDigestIterLength
	v1UUIDLength          = 40
	v1KeySlot1Start       = v1UUIDStart + v1UUIDLength
	v1KeySlot1Length      = 48
	v1KeySlot2Start       = v1KeySlot1Start + v1KeySlot1Length
	v1KeySlot2Length      = 48
	v1KeySlot3Start       = v1KeySlot2Start + v1KeySlot2Length
	v1KeySlot3Length      = 48
	v1KeySlot4Start       = v1KeySlot3Start + v1KeySlot3Length
	v1KeySlot4Length      = 48
	v1KeySlot5Start       = v1KeySlot4Start + v1KeySlot4Length
	v1KeySlot5Length      = 48
	v1KeySlot6Start       = v1KeySlot5Start + v1KeySlot5Length
	v1KeySlot6Length      = 48
	v1KeySlot7Start       = v1KeySlot6Start + v1KeySlot6Length
	v1KeySlot7Length      = 48
	v1KeySlot8Start       = v1KeySlot7Start + v1KeySlot7Length
	v1KeySlot8Length      = 48
	v1HeaderStructSize    = v1KeySlot8Start + v1KeySlot8Length

	v1KeySlotActiveStart             = 0
	v1KeySlotActiveLength            = 4
	v1KeySlotIterationsStart         = v1KeySlotActiveStart + v1KeySlotActiveLength
	v1KeySlotIterationsLength        = 4
	v1KeySlotSaltStart               = v1KeySlotIterationsStart + v1KeySlotIterationsLength
	v1KeySlotSaltLength              = V1SaltSize
	v1KeySlotKeyMaterialOffsetStart  = v1KeySlotSaltStart + v1KeySlotSaltLength
	v1KeySlotKeyMaterialOffsetLength = 4
	v1KeySlotStripesStart            = v1KeySlotKeyMaterialOffsetStart + v1KeySlotKeyMaterialOffsetLength
	v1KeySlotStripesLength           = 4
	v1KeySlotStructSize              = v1KeySlotStripesStart + v1KeySlotStripesLength

	V1DigestSize               = 20
	V1SaltSize                 = 32
	v1NumKeys                  = 8
	v1KeySlotActiveKeyDisabled = 0x0000dead
	v1KeySlotActiveKeyEnabled  = 0x00ac71f3
	V1Stripes                  = 4000
	V1AlignKeyslots            = 4096
	V1SectorSize               = 512
)

var (
	v1CipherNames = map[string]struct{}{
		"aes":     {},
		"twofish": {},
		"serpent": {},
		"cast5":   {},
		"cast6":   {},
	}
	v1Modes = map[string]struct{}{
		"ecb":                 {},
		"cbc-plain":           {},
		"cbc-essiv:sha1":      {},
		"cbc-essiv:sha256":    {},
		"cbc-essiv:sha512":    {},
		"cbc-essiv:ripemd160": {},
		"xts-plain64":         {},
	}
	v1HashSpecs = map[string]struct{}{
		"sha1":      {},
		"sha256":    {},
		"sha512":    {},
		"ripemd160": {},
	}
)

func (h V1Header) readu2(offset int) uint16 {
	t := uint16(0)
	for i := 0; i < 2; i++ {
		t = (t << 8) + uint16(h[offset+i])
	}
	return t
}

func (h V1Header) readu4(offset int) uint32 {
	t := uint32(0)
	for i := 0; i < 4; i++ {
		t = (t << 8) + uint32(h[offset+i])
	}
	return t
}

func (h *V1Header) writeu2(offset int, value uint16) {
	t := value
	for i := 0; i < 2; i++ {
		h[offset+i] = uint8(uint64(t) & 0xff)
		t >>= 8
	}
}

func (h *V1Header) writeu4(offset int, value uint32) {
	t := value
	for i := 0; i < 4; i++ {
		h[offset+i] = uint8(uint32(t) & 0xff)
		t >>= 8
	}
}

func (h V1Header) Magic() string {
	return trimZeroPad(string(h[v1MagicStart : v1MagicStart+v1MagicLength]))
}

func (h *V1Header) SetMagic(magic string) error {
	switch magic {
	case V1Magic:
		copy(h[v1MagicStart:v1MagicStart+v1MagicLength], []uint8(magic))
		return nil
	}
	return fmt.Errorf("magic %q not acceptable, only %q is an acceptable magic value: %w", magic, V1Magic, syscall.EINVAL)
}

func (h V1Header) Version() uint16 {
	return h.readu2(v1VersionStart)
}

func (h *V1Header) SetVersion(version uint16) error {
	switch version {
	case 1:
		h.writeu2(v1VersionStart, version)
		return nil
	}
	return fmt.Errorf("version %d not acceptable, only 1 is an acceptable version: %w", version, syscall.EINVAL)
}

func (h *V1Header) setZeroString(offset int, value string, length int) {
	for len(value) < length {
		value = value + "\000"
	}
	copy(h[offset:offset+length], []uint8(value))
}

func (h *V1Header) setInt8(offset int, s []uint8, length int) {
	for len(s) < length {
		s = append(s, 0)
	}
	copy(h[offset:offset+length], s)
}

func (h V1Header) CipherName() string {
	return trimZeroPad(string(h[v1CipherNameStart : v1CipherNameStart+v1CipherNameLength]))
}

func (h *V1Header) SetCipherName(name string) {
	h.setZeroString(v1CipherNameStart, name, v1CipherNameLength)
}

func (h V1Header) CipherMode() string {
	return trimZeroPad(string(h[v1CipherModeStart : v1CipherModeStart+v1CipherModeLength]))
}

func (h *V1Header) SetCipherMode(mode string) {
	h.setZeroString(v1CipherModeStart, mode, v1CipherModeLength)
}

func (h V1Header) HashSpec() string {
	return trimZeroPad(string(h[v1HashSpecStart : v1HashSpecStart+v1HashSpecLength]))
}

func (h *V1Header) SetHashSpec(spec string) {
	h.setZeroString(v1HashSpecStart, spec, v1HashSpecLength)
}

func (h V1Header) PayloadOffset() uint32 {
	return h.readu4(v1PayloadOffsetStart)
}

func (h *V1Header) SetPayloadOffset(offset uint32) {
	h.writeu4(v1PayloadOffsetStart, offset)
}

func (h V1Header) KeyBytes() uint32 {
	return h.readu4(v1KeyBytesStart)
}

func (h *V1Header) SetKeyBytes(bytes uint32) {
	h.writeu4(v1KeyBytesStart, bytes)
}

func (h *V1Header) KeySlot(slot int) (V1KeySlot, error) {
	var ks V1KeySlot
	if slot < 0 || slot >= v1NumKeys {
		return ks, fmt.Errorf("invalid key slot number (must be 0..%d)", v1NumKeys-1)
	}
	switch slot {
	case 0:
		copy(ks[:], h[v1KeySlot1Start:v1KeySlot1Start+v1KeySlot1Length])
	case 1:
		copy(ks[:], h[v1KeySlot2Start:v1KeySlot2Start+v1KeySlot2Length])
	case 2:
		copy(ks[:], h[v1KeySlot3Start:v1KeySlot3Start+v1KeySlot3Length])
	case 3:
		copy(ks[:], h[v1KeySlot4Start:v1KeySlot4Start+v1KeySlot4Length])
	case 4:
		copy(ks[:], h[v1KeySlot5Start:v1KeySlot5Start+v1KeySlot5Length])
	case 5:
		copy(ks[:], h[v1KeySlot6Start:v1KeySlot6Start+v1KeySlot6Length])
	case 6:
		copy(ks[:], h[v1KeySlot7Start:v1KeySlot7Start+v1KeySlot7Length])
	case 7:
		copy(ks[:], h[v1KeySlot8Start:v1KeySlot8Start+v1KeySlot8Length])
	}
	return ks, nil
}

func (h *V1Header) SetKeySlot(slot int, ks V1KeySlot) error {
	if slot < 0 || slot >= v1NumKeys {
		return fmt.Errorf("invalid key slot number (must be 0..%d)", v1NumKeys-1)
	}
	switch slot {
	case 0:
		copy(h[v1KeySlot1Start:v1KeySlot1Start+v1KeySlot1Length], ks[:])
	case 1:
		copy(h[v1KeySlot2Start:v1KeySlot2Start+v1KeySlot2Length], ks[:])
	case 2:
		copy(h[v1KeySlot3Start:v1KeySlot3Start+v1KeySlot3Length], ks[:])
	case 3:
		copy(h[v1KeySlot4Start:v1KeySlot4Start+v1KeySlot4Length], ks[:])
	case 4:
		copy(h[v1KeySlot5Start:v1KeySlot5Start+v1KeySlot5Length], ks[:])
	case 5:
		copy(h[v1KeySlot6Start:v1KeySlot6Start+v1KeySlot6Length], ks[:])
	case 6:
		copy(h[v1KeySlot7Start:v1KeySlot7Start+v1KeySlot7Length], ks[:])
	case 7:
		copy(h[v1KeySlot8Start:v1KeySlot8Start+v1KeySlot8Length], ks[:])
	}
	return nil
}

func (h V1Header) MKDigest() []uint8 {
	return dupInt8(h[v1MKDigestStart : v1MKDigestStart+v1MKDigestLength])
}

func (h *V1Header) SetMKDigest(digest []uint8) {
	h.setInt8(v1MKDigestStart, digest, v1MKDigestLength)
}

func (h V1Header) MKDigestSalt() []uint8 {
	return dupInt8(h[v1MKDigestSaltStart : v1MKDigestSaltStart+v1MKDigestSaltLength])
}

func (h *V1Header) SetMKDigestSalt(salt []uint8) {
	h.setInt8(v1MKDigestSaltStart, salt, v1MKDigestSaltLength)
}

func (h V1Header) MKDigestIter() uint32 {
	return h.readu4(v1MKDigestIterStart)
}

func (h *V1Header) SetMKDigestIter(bytes uint32) {
	h.writeu4(v1MKDigestIterStart, bytes)
}

func (h V1Header) UUID() string {
	return trimZeroPad(string(h[v1UUIDStart : v1UUIDStart+v1UUIDLength]))
}

func (h *V1Header) SetUUID(uuid string) {
	h.setZeroString(v1UUIDStart, uuid, v1UUIDLength)
}

func (s V1KeySlot) readu4(offset int) uint32 {
	t := uint32(0)
	for i := 0; i < 4; i++ {
		t = (t << 8) + uint32(s[offset+i])
	}
	return t
}

func (s *V1KeySlot) writeu4(offset int, value uint32) {
	t := value
	for i := 0; i < 4; i++ {
		s[offset+i] = uint8(uint32(t) & 0xff)
		t >>= 8
	}
}

func (s *V1KeySlot) setInt8(offset int, i []uint8, length int) {
	for len(s) < length {
		i = append(i, 0)
	}
	copy(s[offset:offset+length], i)
}

func (s V1KeySlot) Active() (bool, error) {
	active := s.readu4(v1KeySlotActiveStart)
	switch active {
	case v1KeySlotActiveKeyDisabled:
		return false, nil
	case v1KeySlotActiveKeyEnabled:
		return true, nil
	}
	return false, fmt.Errorf("got invalid active value %#0x: %w", active, syscall.EINVAL)
}

func (s *V1KeySlot) SetActive(active bool) {
	if active {
		s.writeu4(v1KeySlotActiveStart, v1KeySlotActiveKeyEnabled)
		return
	}
	s.writeu4(v1KeySlotActiveStart, v1KeySlotActiveKeyDisabled)
}

func (s V1KeySlot) Iterations() uint32 {
	return s.readu4(v1KeySlotIterationsStart)
}

func (s *V1KeySlot) SetIterations(iterations uint32) {
	s.writeu4(v1KeySlotIterationsStart, iterations)
}

func (s V1KeySlot) KeySlotSalt() []uint8 {
	return dupInt8(s[v1KeySlotSaltStart : v1KeySlotSaltStart+v1KeySlotSaltLength])
}

func (s *V1KeySlot) SetKeySlotSalt(salt []uint8) {
	s.setInt8(v1KeySlotSaltStart, salt, v1KeySlotSaltLength)
}

func (s V1KeySlot) KeyMaterialOffset() uint32 {
	return s.readu4(v1KeySlotKeyMaterialOffsetStart)
}

func (s *V1KeySlot) SetKeyMaterialOffset(material uint32) {
	s.writeu4(v1KeySlotKeyMaterialOffsetStart, material)
}

func (s V1KeySlot) Stripes() uint32 {
	return s.readu4(v1KeySlotStripesStart)
}

func (s *V1KeySlot) SetStripes(stripes uint32) {
	s.writeu4(v1KeySlotStripesStart, stripes)
}

func CreateV1() ([]byte, error) {
	salt := make([]byte, V1SaltSize)
	n, err := rand.Read(salt)
	if err != nil {
		return nil, err
	}
	if n != len(salt) {
		return nil, errors.New("short read")
	}
	ksSalt := make([]byte, v1KeySlotSaltLength*8)
	n, err = rand.Read(ksSalt)
	if err != nil {
		return nil, err
	}
	if n != len(ksSalt) {
		return nil, errors.New("short read")
	}

	var h V1Header
	h.SetMagic(V1Magic)
	h.SetVersion(1)
	h.SetCipherName("aes")
	h.SetCipherMode("xts-plain64")
	h.SetHashSpec("sha256")
	h.SetKeyBytes(32)
	h.SetMKDigestSalt(salt)
	h.SetMKDigestIter(4000)
	h.SetUUID("")
	for i := 0; i < 7; i++ {
		var keyslot V1KeySlot
		keyslot.SetActive(false)
		keyslot.SetIterations(h.MKDigestIter())
		keyslot.SetStripes(4000)
		keyslot.SetKeySlotSalt(ksSalt[i*v1KeySlotSaltLength : (i+1)*v1KeySlotSaltLength])
		h.SetKeySlot(i, keyslot)
	}
	return nil, nil
}
