/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/asn1"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/hellobchain/newcryptosm/ecdsa"
	"github.com/hellobchain/newcryptosm/sm2"
	x5092 "github.com/hellobchain/newcryptosm/x509"
)

// struct to hold info required for PKCS#8
type pkcs8Info struct {
	Version             int
	PrivateKeyAlgorithm []asn1.ObjectIdentifier
	PrivateKey          []byte
}

type ecPrivateKey struct {
	Version       int
	PrivateKey    []byte
	NamedCurveOID asn1.ObjectIdentifier `asn1:"optional,explicit,tag:0"`
	PublicKey     asn1.BitString        `asn1:"optional,explicit,tag:1"`
}

var (
	oidNamedCurveP224 = asn1.ObjectIdentifier{1, 3, 132, 0, 33}
	oidNamedCurveP256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7}
	oidNamedCurveP384 = asn1.ObjectIdentifier{1, 3, 132, 0, 34}
	oidNamedCurveP521 = asn1.ObjectIdentifier{1, 3, 132, 0, 35}
	oidNamedCurveSM2  = asn1.ObjectIdentifier{1, 2, 156, 10197, 1, 301}
)

var oidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
var oidPublicKeySM2 = asn1.ObjectIdentifier{1, 2, 156, 197, 1, 301}

func oidPublicKeyFromNamedCurve(curve elliptic.Curve) (asn1.ObjectIdentifier, bool) {
	switch curve {
	case elliptic.P224():
		return oidPublicKeyECDSA, true
	case elliptic.P384():
		return oidPublicKeyECDSA, true
	case elliptic.P521():
		return oidPublicKeyECDSA, true
	case elliptic.P256():
		return oidPublicKeyECDSA, true
	case sm2.SM2():
		return oidPublicKeySM2, true
	}
	return nil, false
}

//IsSM2Curve KEY是否为SM2
func IsSM2Curve(pubKey *ecdsa.PublicKey) bool {
	if pubKey.Curve.Params().Name == sm2.SM2().Params().Name {
		return true
	} else {
		return false
	}
}

func oidFromNamedCurve(curve elliptic.Curve) (asn1.ObjectIdentifier, bool) {
	switch curve {
	case sm2.SM2():
		return oidNamedCurveSM2, true
	case elliptic.P224():
		return oidNamedCurveP224, true
	case elliptic.P256():
		return oidNamedCurveP256, true
	case elliptic.P384():
		return oidNamedCurveP384, true
	case elliptic.P521():
		return oidNamedCurveP521, true
	}
	return nil, false
}

// PrivateKeyToDER marshals a private key to der
func PrivateKeyToDER(privateKey interface{}) ([]byte, error) {
	if privateKey == nil {
		return nil, errors.New("Invalid ecdsa private key. It must be different from nil.")
	}

	switch privateKey.(type) {
	case *ecdsa.PrivateKey:
		if ecdsa.IsSM2(privateKey.(*ecdsa.PrivateKey).PublicKey.Curve.Params()) {
			return x5092.MarshalECPrivateKey(privateKey.(*ecdsa.PrivateKey))
		} else { //fc  to be confirm
			return x5092.MarshalECPrivateKey(privateKey.(*ecdsa.PrivateKey))
		}
	default:
		return x5092.MarshalECPrivateKey(privateKey.(*ecdsa.PrivateKey))
	}

}

// PrivateKeyToPEM converts the private key to PEM format.
// EC private keys are converted to PKCS#8 format.
// RSA private keys are converted to PKCS#1 format.
func PrivateKeyToPEM(privateKey interface{}, pwd []byte) ([]byte, error) {
	// Validate inputs
	if len(pwd) != 0 {
		return PrivateKeyToEncryptedPEM(privateKey, pwd)
	}
	if privateKey == nil {
		return nil, errors.New("Invalid key. It must be different from nil.")
	}

	switch k := privateKey.(type) {
	case *ecdsa.PrivateKey:
		if k == nil {
			return nil, errors.New("Invalid ecdsa private key. It must be different from nil.")
		}

		// get the oid for the curve
		oidNamedCurve, ok := oidFromNamedCurve(k.Curve)
		if !ok {
			return nil, errors.New("unknown elliptic curve")
		}

		// based on https://golang.org/src/crypto/x509/sec1.go
		privateKeyBytes := k.D.Bytes()
		paddedPrivateKey := make([]byte, (k.Curve.Params().N.BitLen()+7)/8)
		copy(paddedPrivateKey[len(paddedPrivateKey)-len(privateKeyBytes):], privateKeyBytes)
		// omit NamedCurveOID for compatibility as it's optional
		asn1Bytes, err := asn1.Marshal(ecPrivateKey{
			Version:    1,
			PrivateKey: paddedPrivateKey,
			PublicKey:  asn1.BitString{Bytes: elliptic.Marshal(k.Curve, k.X, k.Y)},
		})

		if err != nil {
			return nil, fmt.Errorf("error marshaling EC key to asn1 [%s]", err)
		}

		var pkcs8Key pkcs8Info
		pkcs8Key.Version = 0
		pkcs8Key.PrivateKeyAlgorithm = make([]asn1.ObjectIdentifier, 2)
		oidPublicKey, _ := oidPublicKeyFromNamedCurve(k.Curve)
		pkcs8Key.PrivateKeyAlgorithm[0] = oidPublicKey
		pkcs8Key.PrivateKeyAlgorithm[1] = oidNamedCurve
		pkcs8Key.PrivateKey = asn1Bytes

		pkcs8Bytes, err := asn1.Marshal(pkcs8Key)
		if err != nil {
			return nil, fmt.Errorf("error marshaling EC key to asn1 [%s]", err)
		}
		return pem.EncodeToMemory(
			&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: pkcs8Bytes,
			},
		), nil
	case *rsa.PrivateKey:
		if k == nil {
			return nil, errors.New("Invalid rsa private key. It must be different from nil.")
		}
		raw := x5092.MarshalPKCS1PrivateKey(k)

		return pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PRIVATE KEY",
				Bytes: raw,
			},
		), nil
	default:
		return nil, errors.New("Invalid key type. It must be *ecdsa.PrivateKey or *rsa.PrivateKey")
	}
}

// PrivateKeyToEncryptedPEM converts a private key to an encrypted PEM
func PrivateKeyToEncryptedPEM(privateKey interface{}, pwd []byte) ([]byte, error) {
	if privateKey == nil {
		return nil, errors.New("Invalid private key. It must be different from nil.")
	}

	switch k := privateKey.(type) {
	case *ecdsa.PrivateKey:
		if k == nil {
			return nil, errors.New("Invalid ecdsa private key. It must be different from nil.")
		}
		raw, err := x5092.MarshalECPrivateKey(k)

		if err != nil {
			return nil, err
		}

		block, err := x5092.EncryptPEMBlock(
			rand.Reader,
			"PRIVATE KEY",
			raw,
			pwd,
			x5092.PEMCipherAES256)

		if err != nil {
			return nil, err
		}

		return pem.EncodeToMemory(block), nil

	default:
		return nil, errors.New("Invalid key type. It must be *ecdsa.PrivateKey")
	}
}

// DERToPrivateKey unmarshals a der to private key
func DERToPrivateKey(der []byte) (key interface{}, err error) {

	if key, err = x5092.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}

	if key, err = x5092.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}

	if key, err = x5092.ParsePKCS8PrivateKey(der); err == nil {
		switch key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey:
			return
		default:
			return nil, errors.New("Found unknown private key type in PKCS#8 wrapping")
		}
	}

	if key, err = x5092.ParsePKCS8PrivateKey(der); err == nil {
		switch key.(type) {
		case *rsa.PrivateKey, *ecdsa.PrivateKey:
			return
		default:
			return nil, errors.New("Found unknown private key type in PKCS#8 wrapping")
		}
	}

	if key, err = x5092.ParseECPrivateKey(der); err == nil {
		return
	}
	if key, err = x5092.ParseECPrivateKey(der); err == nil {
		return
	}

	return nil, errors.New("Invalid key type. The DER must contain an rsa.PrivateKey or ecdsa.PrivateKey")
}

// PEMtoPrivateKey unmarshals a pem to private key
func PEMtoPrivateKey(raw []byte, pwd []byte) (interface{}, error) {
	if len(raw) == 0 {
		return nil, errors.New("Invalid PEM. It must be different from nil.")
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("Failed decoding PEM. Block must be different from nil. [% x]", raw)
	}

	// TODO: derive from header the type of the key

	if x5092.IsEncryptedPEMBlock(block) {
		if len(pwd) == 0 {
			return nil, errors.New("Encrypted Key. Need a password")
		}

		var decrypted []byte
		var err error
		decrypted, err = x5092.DecryptPEMBlock(block, pwd)
		if err != nil {
			decrypted, err = x5092.DecryptPEMBlock(block, pwd)
			if err != nil {
				return nil, fmt.Errorf("Failed PEM decryption [%s]", err)
			}
			return nil, fmt.Errorf("Failed PEM decryption [%s]", err)
		}

		key, err := DERToPrivateKey(decrypted)
		if err != nil {
			return nil, err
		}
		return key, err
	}

	key, err := DERToPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return key, err
}

// PEMtoAES extracts from the PEM an AES key
func PEMtoAES(raw []byte, pwd []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("Invalid PEM. It must be different from nil.")
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("Failed decoding PEM. Block must be different from nil. [% x]", raw)
	}

	if x5092.IsEncryptedPEMBlock(block) {
		if len(pwd) == 0 {
			return nil, errors.New("Encrypted Key. Password must be different fom nil")
		}

		decrypted, err := x5092.DecryptPEMBlock(block, pwd)
		if err != nil {
			return nil, fmt.Errorf("Failed PEM decryption. [%s]", err)
		}
		return decrypted, nil
	}

	return block.Bytes, nil
}

// AEStoPEM encapsulates an AES key in the PEM format
func AEStoPEM(raw []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "AES PRIVATE KEY", Bytes: raw})
}

// AEStoEncryptedPEM encapsulates an AES key in the encrypted PEM format
func AEStoEncryptedPEM(raw []byte, pwd []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("Invalid aes key. It must be different from nil")
	}
	if len(pwd) == 0 {
		return AEStoPEM(raw), nil
	}

	block, err := x5092.EncryptPEMBlock(
		rand.Reader,
		"AES PRIVATE KEY",
		raw,
		pwd,
		x5092.PEMCipherAES256)

	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(block), nil
}

// SM4toPEM encapsulates an SM4 key in the PEM format
func SM4toPEM(raw []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "SM4 PRIVATE KEY", Bytes: raw})
}

// SM4toEncryptedPEM encapsulates an SM4 key in the encrypted PEM format
func SM4toEncryptedPEM(raw []byte, pwd []byte) ([]byte, error) {
	if len(raw) == 0 {
		return nil, errors.New("Invalid aes key. It must be different from nil")
	}
	if len(pwd) == 0 {
		return AEStoPEM(raw), nil
	}

	block, err := x5092.EncryptPEMBlock(
		rand.Reader,
		"SM4 PRIVATE KEY",
		raw,
		pwd,
		x5092.PEMCipherAES256)

	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(block), nil
}

// PublicKeyToPEM marshals a public key to the pem format
func PublicKeyToPEM(publicKey interface{}, pwd []byte) ([]byte, error) {
	if len(pwd) != 0 {
		return PublicKeyToEncryptedPEM(publicKey, pwd)
	}

	if publicKey == nil {
		return nil, errors.New("Invalid public key. It must be different from nil.")
	}

	switch k := publicKey.(type) {
	case *ecdsa.PublicKey:
		if k == nil {
			return nil, errors.New("Invalid ecdsa public key. It must be different from nil.")
		}
		PubASN1, err := x5092.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}

		return pem.EncodeToMemory(
			&pem.Block{
				Type:  "PUBLIC KEY",
				Bytes: PubASN1,
			},
		), nil
	case *rsa.PublicKey:
		if k == nil {
			return nil, errors.New("Invalid rsa public key. It must be different from nil.")
		}
		PubASN1, err := x5092.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}

		return pem.EncodeToMemory(
			&pem.Block{
				Type:  "RSA PUBLIC KEY",
				Bytes: PubASN1,
			},
		), nil

	default:
		return nil, errors.New("Invalid key type. It must be *ecdsa.PublicKey or *rsa.PublicKey")
	}
}

// PublicKeyToDER marshals a public key to the der format
func PublicKeyToDER(publicKey interface{}) ([]byte, error) {
	if publicKey == nil {
		return nil, errors.New("Invalid public key. It must be different from nil.")
	}

	switch k := publicKey.(type) {
	case *ecdsa.PublicKey:
		if k == nil {
			return nil, errors.New("Invalid ecdsa public key. It must be different from nil.")
		}
		PubASN1, err := x5092.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}

		return PubASN1, nil

	case *rsa.PublicKey:
		if k == nil {
			return nil, errors.New("Invalid rsa public key. It must be different from nil.")
		}
		PubASN1, err := x5092.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}

		return PubASN1, nil

	default:
		return nil, errors.New("Invalid key type. It must be *ecdsa.PublicKey or *rsa.PublicKey")
	}
}

// PublicKeyToEncryptedPEM converts a public key to encrypted pem
func PublicKeyToEncryptedPEM(publicKey interface{}, pwd []byte) ([]byte, error) {
	if publicKey == nil {
		return nil, errors.New("Invalid public key. It must be different from nil.")
	}
	if len(pwd) == 0 {
		return nil, errors.New("Invalid password. It must be different from nil.")
	}

	switch k := publicKey.(type) {
	case *ecdsa.PublicKey:
		if k == nil {
			return nil, errors.New("Invalid ecdsa public key. It must be different from nil.")
		}
		raw, err := x5092.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}

		block, err := x5092.EncryptPEMBlock(
			rand.Reader,
			"PUBLIC KEY",
			raw,
			pwd,
			x5092.PEMCipherAES256)

		if err != nil {
			return nil, err
		}

		return pem.EncodeToMemory(block), nil

	default:
		return nil, errors.New("Invalid key type. It must be *ecdsa.PublicKey")
	}
}

// PEMtoPublicKey unmarshals a pem to public key
func PEMtoPublicKey(raw []byte, pwd []byte) (interface{}, error) {
	if len(raw) == 0 {
		return nil, errors.New("Invalid PEM. It must be different from nil.")
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("Failed decoding. Block must be different from nil. [% x]", raw)
	}

	// TODO: derive from header the type of the key
	if x5092.IsEncryptedPEMBlock(block) {
		if len(pwd) == 0 {
			return nil, errors.New("Encrypted Key. Password must be different from nil")
		}

		decrypted, err := x5092.DecryptPEMBlock(block, pwd)
		if err != nil {
			return nil, fmt.Errorf("Failed PEM decryption. [%s]", err)
		}

		key, err := DERToPublicKey(decrypted)
		if err != nil {
			return nil, err
		}
		return key, err
	}

	cert, err := DERToPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, err
}

// DERToPublicKey unmarshals a der to public key
func DERToPublicKey(raw []byte) (pub interface{}, err error) {
	if len(raw) == 0 {
		return nil, errors.New("Invalid DER. It must be different from nil.")
	}

	//fc 先用smalgo的 x509加载 der 信息，然后根据 public key的类型返回相应 类型的 key interface
	keySM2, errSM2 := x5092.ParsePKIXPublicKey(raw)
	if errSM2 != nil {
		return nil, err
	}

	switch keySM2.(type) {
	case *ecdsa.PublicKey:

		if ecdsa.IsSM2(keySM2.(*ecdsa.PublicKey).Curve.Params()) {
			return keySM2, nil
		} else {
			pub, err = x5092.ParsePKIXPublicKey(raw)
		}

	default: //case *rsa.PublicKey,  *dsa.PublicKey,
		pub, err = x5092.ParsePKIXPublicKey(raw)
	}

	return pub, err
}
