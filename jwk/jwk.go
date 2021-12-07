// MIT License
//
// Copyright (c) 2021 Iván Szkiba
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package jwk

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"go.k6.io/k6/js/common"
	"gopkg.in/square/go-jose.v2"
)

type Module struct{}

func New() *Module {
	return &Module{}
}

var ErrUnsupportedAlgorithm = errors.New("unsupported algorithm")

func (m *Module) Parse(ctx context.Context, source string) (*jose.JSONWebKey, error) {
	key := &jose.JSONWebKey{}

	if err := key.UnmarshalJSON([]byte(source)); err != nil {
		return nil, err
	}

	return key, nil
}

func (m *Module) ParseKeySet(ctx context.Context, source string) ([]jose.JSONWebKey, error) {
	keyset := &jose.JSONWebKeySet{}

	if err := json.Unmarshal([]byte(source), &keyset); err != nil {
		return nil, err
	}

	return keyset.Keys, nil
}

func bytes(in interface{}) ([]byte, error) {
	if in == nil || reflect.ValueOf(in).IsZero() {
		return nil, nil
	}

	val, err := common.ToBytes(in)
	if err != nil {
		return nil, fmt.Errorf("%w", err)
	}

	return val, nil
}

func (m *Module) Generate(ctx context.Context, algorithm string, seedIn interface{}) (*jose.JSONWebKey, error) {
	alg := strings.ToUpper(algorithm)

	if alg != string(jose.ED25519) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, algorithm)
	}

	seed, err := bytes(seedIn)
	if err != nil {
		return nil, err
	}

	var priv ed25519.PrivateKey

	if seed == nil {
		_, priv, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
	} else {
		priv = ed25519.NewKeyFromSeed(seed)
	}

	return ed25519Adopt(priv, false), nil
}

func (m *Module) Adopt(ctx context.Context, algorithm string, keyIn interface{}, isPublic bool) (*jose.JSONWebKey, error) {
	alg := strings.ToUpper(algorithm)

	switch alg {
	case string(jose.ED25519):
		key, err := bytes(keyIn)
		if err != nil {
			return nil, err
		}
		return ed25519Adopt(key, isPublic), nil
	case string(jose.RSA1_5):
		key, err := bytes(keyIn)
		if err != nil {
			return nil, err
		}
		return rsa15Adopt(key, isPublic)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, algorithm)
	}
}

func ed25519Adopt(in []byte, isPublic bool) *jose.JSONWebKey {
	k := &jose.JSONWebKey{}
	k.Algorithm = string(jose.EdDSA)
	k.Use = "sig"

	var x string

	if isPublic {
		publicKey := ed25519.PublicKey(in)
		k.Key = publicKey
		x = base64.RawURLEncoding.EncodeToString(publicKey)
	} else {
		privateKey := ed25519.PrivateKey(in)
		k.Key = privateKey
		x = base64.RawURLEncoding.EncodeToString(privateKey[ed25519.SeedSize:])
	}

	// workaround of k.Thumbprint() bug
	kid := sha256.Sum256([]byte(fmt.Sprintf(`{"crv":"Ed25519","kty":"OKP","x":"%s"}`, x)))

	k.KeyID = base64.RawURLEncoding.EncodeToString(kid[:])

	return k
}

func rsa15Adopt(in []byte, isPublic bool) (*jose.JSONWebKey, error) {
	k := &jose.JSONWebKey{}
	k.Algorithm = string(jose.RS256)
	k.Use = "sig"

	var x string
	if isPublic {
		publicKey := ed25519.PublicKey(in)
		k.Key = publicKey
		x = base64.RawURLEncoding.EncodeToString(publicKey)
	} else {
		privateKey, err := x509.ParsePKCS1PrivateKey(in)
		if err != nil {
			return nil, err
		}
		k.Key = privateKey
		x = "TODO" //base64.RawURLEncoding.EncodeToString(privateKey.)
	}

	//{
	//  "use": "sig",
	//  "kid": "1",
	//  "kty": "RSA",
	//  "n": "",
	//  "e": "AQAB"
	//}
	// workaround of k.Thumbprint() bug
	//TODO fill for RSA
	kid := sha256.Sum256([]byte(fmt.Sprintf(`{"kty":"RSA"}`, x)))

	k.KeyID = base64.RawURLEncoding.EncodeToString(kid[:])
	return k, nil
}
