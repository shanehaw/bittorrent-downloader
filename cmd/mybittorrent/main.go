package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	// "strings"
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
	} else if command == "peers" {
		lines, err := peers(os.Args[2])
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
	contents, err := readFile(file)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %s", err.Error())
	}

	decoded, _, err := decodeBencode(contents)
	if err != nil {
		return nil, err
	}

	dict, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid bencode")
	}

	baseUrl := dict["announce"].(string)
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
		fmt.Sprintf("Tracker URL: %s", baseUrl),
		fmt.Sprintf("Length: %d", length),
		fmt.Sprintf("Info Hash: %s", hash),
		fmt.Sprintf("Piece Length: %d", piecesLength),
		"Piece Hashes:",
	}
	result = append(result, hashes...)

	return result, nil
}

func readFile(file string) ([]byte, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %s", err.Error())
	}

	contents, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %s", err.Error())
	}
	return contents, nil
}

func peers(file string) ([]string, error) {
	result := []string{}
	contents, err := readFile(file)
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

	baseUrl := dict["announce"].(string)
	info := dict["info"].(map[string]any)
	length := info["length"].(int)
	encodedInfo, err := encodeBencode(info)
	if err != nil {
		return nil, err
	}

	h := sha1.New()
	h.Write(encodedInfo)
	hashBytes := h.Sum(nil)

	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, fmt.Errorf("error parsing base url: %s", err.Error())
	}

	params := url.Values{}
	params.Add("info_hash", string(hashBytes))
	params.Add("peer_id", createUniqueId())
	params.Add("port", "6881")
	params.Add("uploaded", "0")
	params.Add("downloaded", "0")
	params.Add("left", fmt.Sprintf("%d", length))
	params.Add("compact", "1")
	u.RawQuery = params.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating new http get request: %s", err.Error())
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending http request: %s", err.Error())
	}
	defer response.Body.Close()

	responseBodyBytes, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %s", err.Error())
	}

	resp, _, err := decodeBencode(responseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("error failed to decoded response: %s", err.Error())
	}

	respDict := resp.(map[string]any)
	possiblePeers, ok := respDict["peers"]
	if ok {
		peers := []byte(possiblePeers.(string))
		for i := 0; i < len(peers); i += 6 {
			ip0 := peers[i]
			ip1 := peers[i+1]
			ip2 := peers[i+2]
			ip3 := peers[i+3]
			port := binary.BigEndian.Uint16([]byte{peers[i+4], peers[i+5]})
			result = append(result, fmt.Sprintf("%d.%d.%d.%d:%d", ip0, ip1, ip2, ip3, port))
		}
	}

	return result, nil
}

func createUniqueId() string {
	return "79106947871722704741"
}
