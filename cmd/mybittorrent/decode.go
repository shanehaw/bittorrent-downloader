package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	// bencode "github.com/jackpal/bencode-go" // Available if you need it!
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	firstRune := rune(bencodedString[0])
	if unicode.IsDigit(firstRune) {
		return decodeString(bencodedString)
	} else if firstRune == 'i' {
		return decodeInteger(bencodedString)
	} else {
		return "", fmt.Errorf("only strings are supported at the moment")
	}
}

func decodeString(bencodedString string) (string, error) {
	var firstColonIndex int

	for i := 0; i < len(bencodedString); i++ {
		if bencodedString[i] == ':' {
			firstColonIndex = i
			break
		}
	}

	lengthStr := bencodedString[:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", err
	}

	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], nil
}

func decodeInteger(bencodedString string) (int, error) {
	endIndex := strings.Index(bencodedString, "e")
	intPart := bencodedString[1:endIndex]
	return strconv.Atoi(intPart)
}
