package mqttwire

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
)

var (
	oidPBES2          = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 13}
	oidPBKDF2         = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 5, 12}
	oidHMACWithSHA1   = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 7}
	oidHMACWithSHA256 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 9}
	oidHMACWithSHA384 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 10}
	oidHMACWithSHA512 = asn1.ObjectIdentifier{1, 2, 840, 113549, 2, 11}
	oidAES128CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 2}
	oidAES192CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 22}
	oidAES256CBC      = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 1, 42}
)

type algorithm struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue `asn1:"optional"`
}

type encryptedPrivateKeyInfo struct {
	Algorithm     algorithm
	EncryptedData []byte
}

type pbes2Params struct {
	KeyDerivationFunc algorithm
	EncryptionScheme  algorithm
}

type pbkdf2Params struct {
	Salt           []byte
	IterationCount int
	KeyLength      int       `asn1:"optional"`
	PRF            algorithm `asn1:"optional"`
}

// maxIterations bounds the PBKDF2 work a key file can demand.
const maxIterations = 10_000_000

// DecryptPKCS8 decrypts a PKCS#8 EncryptedPrivateKeyInfo (DER) with PBES2:
// PBKDF2 with HMAC-SHA1/256/384/512 and AES-128/192/256-CBC – the scheme of
// `openssl pkcs8 -topk8 -v2 aes-256-cbc` and of the dev PKI (REQ-CNT-008).
// It returns the PKCS#8 PrivateKeyInfo (DER).
func DecryptPKCS8(der, passphrase []byte) ([]byte, error) {
	var info encryptedPrivateKeyInfo
	if rest, err := asn1.Unmarshal(der, &info); err != nil || len(rest) > 0 {
		return nil, errors.New("encrypted private key: no PKCS#8 EncryptedPrivateKeyInfo")
	}
	if !info.Algorithm.Algorithm.Equal(oidPBES2) {
		return nil, fmt.Errorf("encrypted private key: scheme %v is not PBES2", info.Algorithm.Algorithm)
	}
	var p pbes2Params
	if _, err := asn1.Unmarshal(info.Algorithm.Parameters.FullBytes, &p); err != nil {
		return nil, fmt.Errorf("encrypted private key: PBES2 parameters: %w", err)
	}
	if !p.KeyDerivationFunc.Algorithm.Equal(oidPBKDF2) {
		return nil, fmt.Errorf("encrypted private key: key derivation %v is not PBKDF2", p.KeyDerivationFunc.Algorithm)
	}
	var kdf pbkdf2Params
	if _, err := asn1.Unmarshal(p.KeyDerivationFunc.Parameters.FullBytes, &kdf); err != nil {
		return nil, fmt.Errorf("encrypted private key: PBKDF2 parameters: %w", err)
	}
	if kdf.IterationCount < 1 || kdf.IterationCount > maxIterations {
		return nil, fmt.Errorf("encrypted private key: PBKDF2 iteration count %d", kdf.IterationCount)
	}
	var prf func() hash.Hash
	switch o := kdf.PRF.Algorithm; {
	case len(o) == 0 || o.Equal(oidHMACWithSHA1):
		prf = sha1.New
	case o.Equal(oidHMACWithSHA256):
		prf = sha256.New
	case o.Equal(oidHMACWithSHA384):
		prf = sha512.New384
	case o.Equal(oidHMACWithSHA512):
		prf = sha512.New
	default:
		return nil, fmt.Errorf("encrypted private key: PBKDF2 PRF %v not supported", o)
	}
	var keyLen int
	switch o := p.EncryptionScheme.Algorithm; {
	case o.Equal(oidAES128CBC):
		keyLen = 16
	case o.Equal(oidAES192CBC):
		keyLen = 24
	case o.Equal(oidAES256CBC):
		keyLen = 32
	default:
		return nil, fmt.Errorf("encrypted private key: cipher %v not supported (AES-CBC only)", o)
	}
	if kdf.KeyLength != 0 && kdf.KeyLength != keyLen {
		return nil, fmt.Errorf("encrypted private key: key length %d does not fit the cipher", kdf.KeyLength)
	}
	var iv []byte
	if _, err := asn1.Unmarshal(p.EncryptionScheme.Parameters.FullBytes, &iv); err != nil || len(iv) != aes.BlockSize {
		return nil, errors.New("encrypted private key: no AES-CBC IV")
	}
	data := info.EncryptedData
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, errors.New("encrypted private key: data length is no multiple of the block size")
	}
	key, err := pbkdf2.Key(prf, string(passphrase), kdf.Salt, kdf.IterationCount, keyLen)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plain, data)
	pad := int(plain[len(plain)-1])
	if pad == 0 || pad > aes.BlockSize {
		return nil, errWrongPassphrase
	}
	for _, b := range plain[len(plain)-pad:] {
		if int(b) != pad {
			return nil, errWrongPassphrase
		}
	}
	plain = plain[:len(plain)-pad]
	if _, err := x509.ParsePKCS8PrivateKey(plain); err != nil {
		return nil, errWrongPassphrase
	}
	return plain, nil
}

var errWrongPassphrase = errors.New("encrypted private key: wrong passphrase or damaged key")

// keyPEM returns the private key of a PEM file as an unencrypted PEM block
// for tls.X509KeyPair. An `ENCRYPTED PRIVATE KEY` needs the passphrase.
func keyPEM(data, passphrase []byte) ([]byte, error) {
	for rest := data; ; {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			return nil, errors.New("no private key in the key file")
		}
		switch b.Type {
		case "ENCRYPTED PRIVATE KEY":
			if len(passphrase) == 0 {
				return nil, errors.New("the private key is encrypted: a passphrase is needed (--key-passphrase-file)")
			}
			der, err := DecryptPKCS8(b.Bytes, passphrase)
			if err != nil {
				return nil, err
			}
			return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
		case "PRIVATE KEY", "EC PRIVATE KEY", "RSA PRIVATE KEY":
			if _, legacy := b.Headers["DEK-Info"]; legacy {
				return nil, errors.New("legacy PEM encryption (DEK-Info) is not supported; use PKCS#8 PBES2")
			}
			return pem.EncodeToMemory(b), nil
		}
	}
}
