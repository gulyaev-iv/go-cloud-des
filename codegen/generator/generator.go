package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Result struct {
	Hash   string
	Source []byte
}

func Generate(data []byte) (*Result, error) {
	model, err := Parse(data)
	if err != nil {
		return nil, err
	}

	if err := ValidateAndLink(model); err != nil {
		return nil, err
	}

	source, err := emit(model)
	if err != nil {
		return nil, err
	}

	return &Result{
		Hash:   model.Hash,
		Source: source,
	}, nil
}

func GenerateWithHash(hash string, data []byte) (*Result, error) {
	if len(hash) != 64 {
		return nil, fmt.Errorf("incorrect model hash: expected 64 hex characters, got %d", len(hash))
	}

	if !isHex([]byte(hash)) {
		return nil, fmt.Errorf("incorrect model hash: expected hexadecimal SHA-256 hash")
	}

	dataWithHash := appendHashLine(hash, data)
	return Generate(dataWithHash)
}

func GenerateAutoHash(data []byte) (*Result, error) {
	hash := HashDSL(data)
	return GenerateWithHash(hash, data)
}

func HashDSL(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func appendHashLine(hash string, data []byte) []byte {
	result := make([]byte, 0, len(hash)+1+len(data))
	result = append(result, hash...)
	result = append(result, '\n')
	result = append(result, data...)
	return result
}
