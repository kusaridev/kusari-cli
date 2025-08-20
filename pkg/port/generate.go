// Copyright (c) Kusari <https://www.kusari.dev/>
// SPDX-License-Identifier: MIT

package port

import (
	"crypto/rand"
	"math/big"
	"strconv"
)

var GenerateRandomPortOrDefault func() string = _generateRandomPortOrDefault
var generateRandomPort func() (string, error) = _generateRandomPort

func _generateRandomPort() (string, error) {
	min := int64(62001)
	max := int64(62009)

	// Range size
	diff := max - min + 1

	// Generate a secure random number in [0, diff)
	n, err := rand.Int(rand.Reader, big.NewInt(diff))
	if err != nil {
		return "", err
	}

	// Offset by min
	result := min + n.Int64()

	return strconv.FormatInt(result, 10), nil
}

func _generateRandomPortOrDefault() string {
	redirectPort, err := generateRandomPort()
	if err != nil {
		redirectPort = "62009"
	}
	return redirectPort
}
