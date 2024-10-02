package main

import (
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
		file := os.Args[2]
		f, err := os.Open(file)
		if err != nil {
			fmt.Printf("error opening file: %s", err.Error())
			return
		}

		contents, err := io.ReadAll(f)
		if err != nil {
			fmt.Printf("error reading file: %s", err.Error())
			return
		}

		decoded, _, err := decodeBencode(contents)
		if err != nil {
			fmt.Println(err)
			return
		}

		dict, ok := decoded.(map[string]any)
		if !ok {
			fmt.Println("invalid bencode")
			return
		}

		url := dict["announce"].(string)
		length := dict["info"].(map[string]any)["length"].(int)

		fmt.Println("Tracker URL:", url)
		fmt.Println("Length:", length)

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
