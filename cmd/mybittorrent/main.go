package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// Ensures gofmt doesn't remove the "os" encoding/json import (feel free to remove this!)
var _ = json.Marshal

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345

func main() {
	// You can use print statements as follows for debugging, they'll be visible when running tests.
	// fmt.Println("Logs from your program will appear here!")

	command := os.Args[1]

	if command == "decode" {
		// Uncomment this block to pass the first stage
		bencodedValue := os.Args[2]

		decoded, _, err := decodeBencode([]byte(bencodedValue))
		if err != nil {
			fmt.Println(err)
			return
		}

		jsonOutput, _ := json.Marshal(decoded)
		fmt.Println(string(jsonOutput))
	} else if command == "info" {
		lines, err := info(os.Args[2])
		if err != nil {
			fmt.Println(err)
			return
		}

		for _, line := range lines {
			fmt.Println(line)
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}

func info(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %s", err.Error())
	}

	contents, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err.Error())
	}

	decoded, _, err := decodeBencode(contents)
	if err != nil {
		return nil, err
	}

	dict, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid bencode")
	}

	url := dict["announce"].(string)
	info := dict["info"].(map[string]any)
	length := info["length"].(int)

	encodedInfo, err := encodeBencode(info)
	if err != nil {
		return nil, err
	}

	h := sha1.New()
	h.Write(encodedInfo)
	hash := hex.EncodeToString(h.Sum(nil))

	piecesLength := info["piece length"].(int)
	pieces := info["pieces"].(string)
	hashes := []string{}
	for cur := 0; cur < len(pieces); cur += 20 {
		hashes = append(hashes, hex.EncodeToString([]byte(pieces[cur:cur+20])))
	}

	result := []string{
		fmt.Sprintf("Tracker URL: %s", url),
		fmt.Sprintf("Length: %d", length),
		fmt.Sprintf("Info Hash: %s", hash),
		fmt.Sprintf("Piece Length: %d", piecesLength),
		"Piece Hashes:",
	}
	result = append(result, hashes...)

	return result, nil
}
