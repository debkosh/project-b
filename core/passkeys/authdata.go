package passkeys

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
)

// AAGUID names Project B as the maker of a passkey to the sites that ask:
// 81dc8063-9ac3-4b1f-ab00-9e8795c47c60, fixed for good. No attestation goes with it.
var AAGUID = [16]byte{0x81, 0xdc, 0x80, 0x63, 0x9a, 0xc3, 0x4b, 0x1f, 0xab, 0x00, 0x9e, 0x87, 0x95, 0xc4, 0x7c, 0x60}

// The flags of the authenticator data (WebAuthn, 6.1): the person was there, and was
// verified; the passkey can be backed up, as ours sync, and is backed up now; and the
// data holds a new credential.
const (
	flagPresent  = 0x01
	flagVerified = 0x04
	flagCanSync  = 0x08
	flagSynced   = 0x10
	flagAttested = 0x40
)

// coseKey is a P-256 public key as a COSE_Key for ES256 (RFC 9052): kty EC2, alg -7, crv
// P-256, x and y, in CTAP2's canonical CBOR.
func coseKey(public *ecdsa.PublicKey) []byte {
	var x, y [32]byte
	public.X.FillBytes(x[:])
	public.Y.FillBytes(y[:])
	out := []byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}
	out = append(out, x[:]...)
	out = append(out, 0x22, 0x58, 0x20)
	return append(out, y[:]...)
}

// data is the authenticator data all passkeys of ours give: the hash of the site's RP ID,
// the flags, and a signature count that stays 0, as a synced passkey's does. synced says
// sync is on now, so the passkey is on the server too.
func data(rpID string, flags byte, synced bool) []byte {
	hash := sha256.Sum256([]byte(rpID))
	flags |= flagPresent | flagVerified | flagCanSync
	if synced {
		flags |= flagSynced
	}
	return append(append(hash[:], flags), 0, 0, 0, 0)
}

// AssertionData is the authenticator data of a sign-in at the site rpID, which Sign signs
// with the hash of the client data after it.
func AssertionData(rpID string, synced bool) []byte { return data(rpID, 0, synced) }

// AttestedData is the authenticator data of a passkey just made: the same, and then the
// new credential, with our AAGUID, its id and its public key as Make gave them.
func AttestedData(rpID string, id, cose []byte, synced bool) []byte {
	out := append(data(rpID, flagAttested, synced), AAGUID[:]...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(id)))
	return append(append(out, id...), cose...)
}
