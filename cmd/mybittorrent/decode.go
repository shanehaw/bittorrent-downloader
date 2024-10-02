package main

import (
	"encoding/json"
	"errors"
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
func decodeBencode(bencodedString string) (any, int, error) {
	firstRune := rune(bencodedString[0])
	if unicode.IsDigit(firstRune) {
		return decodeString(bencodedString)
	} else if firstRune == 'i' {
		return decodeInteger(bencodedString)
	} else if firstRune == 'l' {
		return decodeList(bencodedString)
	} else {
		return "", -1, fmt.Errorf("unhandled type: '%s'", bencodedString)
	}
}

func decodeString(bencodedString string) (string, int, error) {
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
		return "", -1, err
	}

	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], firstColonIndex + 1 + length, nil
}

func decodeInteger(bencodedString string) (int, int, error) {
	endIndex := strings.Index(bencodedString, "e")
	intPart := bencodedString[1:endIndex]
	v, err := strconv.Atoi(intPart)
	return v, endIndex + 1, err
}

func decodeList(bencodedString string) (any, int, error) {
	result := []any{}
	curIndex := 1 // skip initial 'l'
	for curIndex < len(bencodedString) && rune(bencodedString[curIndex]) != 'e' {
		item, newIndex, err := decodeBencode(bencodedString[curIndex:])
		if err != nil {
			return nil, newIndex, err
		}
		curIndex += newIndex
		result = append(result, item)

		if curIndex >= len(bencodedString) {
			return nil, newIndex, errors.New("reached end of the string before finding the end of the list")
		}
	}

	return result, curIndex + 1, nil
}
