package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const sixteenKilobytes = 16 * 1024
const supportExtensionsMaskByte = 0x10
const ourMetadataExtensionId = 1

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
	} else if command == "handshake" {
		lines, err := performHandshake(os.Args[2], os.Args[3])
		if err != nil {
			fmt.Println(err)
			return
		}

		for _, line := range lines {
			fmt.Println(line)
		}
	} else if command == "download_piece" {
		pieceIndex, err := strconv.Atoi(os.Args[5])
		if err != nil {
			fmt.Printf("failed to parse piece index: %s\n", err.Error())
		}
		if err = downloadPiece(os.Args[3], os.Args[4], pieceIndex); err != nil {
			fmt.Printf("failed to download piece: %s\n", err.Error())
			os.Exit(1)
		}
	} else if command == "download" {
		if err := downloadFile(os.Args[3], os.Args[4]); err != nil {
			fmt.Printf("failed to download file: %s\n", err.Error())
			os.Exit(1)
		}
	} else if command == "magnet_parse" {
		if err := magnet_parse(os.Args[2]); err != nil {
			fmt.Printf("failed to parse magnet: %s\n", err.Error())
			os.Exit(1)
		}
	} else if command == "magnet_handshake" {
		if err := magnet_handshake(os.Args[2]); err != nil {
			fmt.Printf("failed to perform magnet handshake: %s\n", err.Error())
			os.Exit(1)
		}
	} else if command == "magnet_info" {
		if err := magnet_info(os.Args[2]); err != nil {
			fmt.Printf("failed to get magnet info: %s\n", err.Error())
			os.Exit(1)
		}
	} else if command == "magnet_download_piece" {
		pieceIndex, err := strconv.Atoi(os.Args[5])
		if err != nil {
			fmt.Printf("failed to parse piece index: %s\n", err.Error())
		}
		if err := magnet_download_piece(os.Args[3], os.Args[4], pieceIndex); err != nil {
			fmt.Printf("failed to get magnet info: %s\n", err.Error())
			os.Exit(1)
		}
	} else if command == "magnet_download" {
		if err := magnet_download(os.Args[3], os.Args[4]); err != nil {
			fmt.Printf("failed to download file: %s\n", err.Error())
			os.Exit(1)
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

	hashBytes, err := getInfoHash(info)
	if err != nil {
		return nil, err
	}

	responseBodyBytes, err := sendRequest(baseUrl, hashBytes, length)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %s", err.Error())
	}

	resp, _, err := decodeBencode(responseBodyBytes)
	if err != nil {
		return nil, fmt.Errorf("error failed to decoded response: %s", err.Error())
	}

	respDict := resp.(map[string]any)
	peers := getPeers(respDict)
	result = append(result, peers...)

	return result, nil
}

func getInfoHash(info map[string]any) ([]byte, error) {
	encodedInfo, err := encodeBencode(info)
	if err != nil {
		return nil, err
	}

	h := sha1.New()
	h.Write(encodedInfo)
	hashBytes := h.Sum(nil)
	return hashBytes, nil
}

func sendRequest(trackerURL string, infoHash []byte, length int) ([]byte, error) {
	u, err := url.Parse(trackerURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing base url: %s", err.Error())
	}

	params := url.Values{}
	params.Add("info_hash", string(infoHash))
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

	return responseBodyBytes, nil
}

func getPeers(dict map[string]any) []string {
	peers := []string{}
	possiblePeers, ok := dict["peers"]
	if ok {
		peersBytes := []byte(possiblePeers.(string))
		for i := 0; i < len(peersBytes); i += 6 {
			ip0 := peersBytes[i]
			ip1 := peersBytes[i+1]
			ip2 := peersBytes[i+2]
			ip3 := peersBytes[i+3]
			port := binary.BigEndian.Uint16([]byte{peersBytes[i+4], peersBytes[i+5]})
			peers = append(peers, fmt.Sprintf("%d.%d.%d.%d:%d", ip0, ip1, ip2, ip3, port))
		}
	}
	return peers
}

func createUniqueId() string {
	return "79106947871722704741"
}

type handshake struct {
	infoHash          []byte
	peerID            []byte
	supportExtensions bool
}

// 161.35.46.221:51414
func performHandshake(file, peerConnectionString string) ([]string, error) {
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

	info := dict["info"].(map[string]any)
	encodedInfo, err := encodeBencode(info)
	if err != nil {
		return nil, err
	}

	h := sha1.New()
	h.Write(encodedInfo)
	hashBytes := h.Sum(nil)

	hs := handshake{
		infoHash: hashBytes,
		peerID:   []byte("00112233445566778899"),
	}

	responseHandshake, err := doHandshakeWithPeer(peerConnectionString, &hs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response handshake: %s", err.Error())
	}

	result = append(result, fmt.Sprintf("Peer ID: %s", hex.EncodeToString(responseHandshake.peerID)))
	return result, nil
}

func (hs *handshake) makeMessage() []byte {
	message := []byte{}
	message = append(message, []byte{19}...)
	message = append(message, []byte("BitTorrent protocol")...)
	if hs.supportExtensions {
		message = append(message, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00}...)
	} else {
		message = append(message, []byte{0, 0, 0, 0, 0, 0, 0, 0}...)
	}
	message = append(message, hs.infoHash...)
	message = append(message, []byte(hs.peerID)...)
	return message
}

func (hs *handshake) makeExtendedMessage() []byte {
	message := []byte{}
	message = append(message, []byte{19}...)
	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x00}...)
	message = append(message, hs.infoHash...)
	message = append(message, []byte(hs.peerID)...)
	return message
}

func (hs *handshake) parseMessage(message []byte) error {
	if len(message) < 68 {
		return fmt.Errorf("message was too small")
	}
	reservedBytes := message[20:28]
	// fmt.Println("reservedBytes", reservedBytes)
	// fmt.Println("reservedBytes[5]", reservedBytes[5])
	if reservedBytes[5]&supportExtensionsMaskByte == supportExtensionsMaskByte {
		hs.supportExtensions = true
	}
	hs.infoHash = message[28:48]
	hs.peerID = message[48:68]
	return nil
}

func downloadPiece(targetLocation, file string, pieceIndex int) error {
	contents, err := readFile(file)
	if err != nil {
		return fmt.Errorf("error reading file: %s", err.Error())
	}

	decoded, _, err := decodeBencode(contents)
	if err != nil {
		return err
	}

	dict, ok := decoded.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid bencode")
	}

	baseUrl := dict["announce"].(string)
	info := dict["info"].(map[string]any)
	fileLength := info["length"].(int)
	pieceLength := info["piece length"].(int)
	fmt.Println(string(contents))
	pieces := info["pieces"].(string)
	hashByIndex := calcPieceHashes(pieces)

	infoHashBytes, err := getInfoHash(info)
	if err != nil {
		return err
	}

	responseBodyBytes, err := sendRequest(baseUrl, infoHashBytes, fileLength)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err.Error())
	}

	resp, _, err := decodeBencode(responseBodyBytes)
	if err != nil {
		return fmt.Errorf("error failed to decoded response: %s", err.Error())
	}

	respDict := resp.(map[string]any)
	peers := getPeers(respDict)
	if len(peers) < 1 {
		return fmt.Errorf("did not receive enough peers")
	}

	peer := peers[0]
	hs := handshake{
		infoHash: infoHashBytes,
		peerID:   createRandomID(),
	}

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return fmt.Errorf("failed to connect via tcp to peer: %s", err.Error())
	}
	defer conn.Close()

	_, err = doHandshakeOnConnection(conn, &hs)
	if err != nil {
		return fmt.Errorf("failed to do handshake with peer: %s", err.Error())
	}

	response, err := waitForNextMessage(conn)
	if err != nil {
		return fmt.Errorf("failed wait for new message: %s", err.Error())
	}
	_, _ = parseMessage(response)

	response, err = sendInterested(conn)
	if err != nil {
		return fmt.Errorf("failed send interested message: %s", err.Error())
	}
	_, _ = parseMessage(response)

	actualPieceLength := getPieceLengthForIndex(fileLength, pieceLength, pieceIndex)
	expectedBlocks := calcExpectedBlocks(actualPieceLength)

	currentOffset := 0
	blocks := [][]byte{}
	for i := 0; i < expectedBlocks; i++ {
		requestLength := int(math.Min(float64(sixteenKilobytes), float64(actualPieceLength-currentOffset)))
		message := createRequestMessage(pieceIndex, currentOffset, requestLength)

		_, err := conn.Write(message)
		if err != nil {
			return fmt.Errorf("failed to read response after request message: %s", err.Error())
		}

		resp, err := readExactLength(conn, requestLength+13)
		if err != nil {
			return fmt.Errorf("failed to read piece message: %s", err.Error())
		}

		_, _, _, block := parsePieceMessage(resp)
		blocks = append(blocks, block)
		currentOffset += requestLength
	}

	newPiece := []byte{}
	for _, b := range blocks {
		newPiece = append(newPiece, b...)
	}

	pieceHash, err := hashBytesNew(newPiece)
	if err != nil {
		return fmt.Errorf("failed to generate hash for new piece")
	}

	expectedHash := hashByIndex[pieceIndex]
	fmt.Println("eHash", expectedHash)
	fmt.Println("aHash", pieceHash)
	if pieceHash != expectedHash {
		return fmt.Errorf("piece hash did not match hash in torrent file. actual: %s, expected: %s", pieceHash, expectedHash)
	}

	if err = os.WriteFile(targetLocation, newPiece, 0666); err != nil {
		return fmt.Errorf("failed to open temp file to write piece: %s", err.Error())
	}

	return nil
}

func calcPieceHashes(rawPieces string) map[int]string {
	hashByIndex := make(map[int]string)
	index := 0
	for cur := 0; cur < len(rawPieces); cur += 20 {
		h := hex.EncodeToString([]byte(rawPieces[cur : cur+20]))
		hashByIndex[index] = h
		index++
	}
	return hashByIndex
}

func calcExpectedBlocks(pieceLength int) int {
	expectedBlocks := pieceLength / sixteenKilobytes
	if pieceLength%sixteenKilobytes > 0 {
		expectedBlocks++
	}
	return expectedBlocks
}

func getPieceLengthForIndex(fileLength, pieceLength, pieceIndex int) int {
	pieceLengthsByIndex := calcPieceLengthMap(fileLength, pieceLength)
	return pieceLengthsByIndex[pieceIndex]
}

func calcPieceLengthMap(fileLength, pieceLength int) map[int]int {
	result := make(map[int]int)
	currentIndex := 0
	current := 0
	for current+pieceLength <= fileLength {
		result[currentIndex] = pieceLength
		current += pieceLength
		currentIndex++
	}
	result[currentIndex] = fileLength - current
	return result
}

func hashBytesNew(obj []byte) (string, error) {
	h := sha1.New()
	_, err := h.Write(obj)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func createRandomID() []byte {
	return []byte(randomString(20))
}

func randomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = byte(rand.Intn(10) + 48)
	}
	return string(b)
}

func doHandshakeWithPeer(peerConnectionString string, start *handshake) (*handshake, error) {
	conn, err := net.Dial("tcp", peerConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to connect via tcp to peer: %s", err.Error())
	}
	defer conn.Close()
	return doHandshakeOnConnection(conn, start)
}

func doHandshakeOnConnection(conn net.Conn, start *handshake) (*handshake, error) {
	message := start.makeMessage()
	_, err := conn.Write(message)
	if err != nil {
		return nil, fmt.Errorf("failed to write to tcp connection: %s", err.Error())
	}

	finalResponse, err := readExactLength(conn, 68)
	if err != nil {
		return nil, fmt.Errorf("failed to read from tcp connection: %s", err.Error())
	}

	// fmt.Println("peer handshake response", finalResponse)

	responseHandshake := &handshake{}
	if err := responseHandshake.parseMessage(finalResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response handshake: %s", err.Error())
	}

	return responseHandshake, nil
}

func readExactLength(conn net.Conn, size int) ([]byte, error) {
	result := []byte{}
	numOfZeroReads := 0
	for len(result) < size {
		buf := make([]byte, size-len(result))
		n, err := conn.Read(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to read from tcp connection: %s", err.Error())
		}
		if n == 0 {
			numOfZeroReads++
			if numOfZeroReads > 10 {
				return nil, fmt.Errorf("failed to read from tcp connection: no data")
			}
		}
		result = append(result, buf[:n]...)
	}
	return result, nil
}

func waitForNextMessage(conn net.Conn) ([]byte, error) {
	result := []byte{}
	buffer := make([]byte, 1024)
	n, err := conn.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read from tcp connection: %s", err.Error())
	}
	result = append(result, buffer[:n]...)
	for n == 1024 {
		time.Sleep(10 * time.Millisecond)
		n, err = conn.Read(buffer)
		if err != nil {
			return nil, fmt.Errorf("failed to read from tcp connection: %s", err.Error())
		}
		result = append(result, buffer[:n]...)
	}
	return result, nil
}

func sendInterested(conn net.Conn) ([]byte, error) {
	message := []byte{}
	message = binary.BigEndian.AppendUint32(message, uint32(1))
	message = append(message, byte(2))
	_, err := conn.Write(message)
	if err != nil {
		return nil, fmt.Errorf("failed to write interested message")
	}

	return waitForNextMessage(conn)
}

func sendMessageAndWaitForResponse(conn net.Conn, message []byte) ([]byte, error) {
	_, err := conn.Write(message)
	if err != nil {
		return nil, fmt.Errorf("failed to write interested message")
	}

	return waitForNextMessage(conn)
}

func parseMessage(message []byte) (byte, []byte) {
	length := binary.BigEndian.Uint32(message[:4])
	id := message[4]
	rest := []byte{}
	if length > 1 {
		rest = message[5:]
	}
	return id, rest
}

func parsePieceMessage(message []byte) (byte, uint32, uint32, []byte) {
	length := binary.BigEndian.Uint32(message[:4])
	id := message[4]
	index := binary.BigEndian.Uint32(message[5:9])
	begin := binary.BigEndian.Uint32(message[9:13])
	block := []byte{}
	if length > 1 {
		block = message[13:]
	}
	return id, index, begin, block
}

func createRequestMessage(index, begin, length int) []byte {
	message := []byte{}
	message = binary.BigEndian.AppendUint32(message, uint32(13))
	message = append(message, byte(6))
	message = binary.BigEndian.AppendUint32(message, uint32(index))
	message = binary.BigEndian.AppendUint32(message, uint32(begin))
	message = binary.BigEndian.AppendUint32(message, uint32(length))
	return message
}

func downloadFile(downloadTarget, file string) error {
	contents, err := readFile(file)
	if err != nil {
		return fmt.Errorf("error reading file: %s", err.Error())
	}

	fmt.Println(string(contents))
	decoded, _, err := decodeBencode(contents)
	if err != nil {
		return err
	}

	dict, ok := decoded.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid bencode")
	}

	baseUrl := dict["announce"].(string)
	info := dict["info"].(map[string]any)
	fileLength := info["length"].(int)
	pieceLength := info["piece length"].(int)
	pieces := info["pieces"].(string)
	hashByIndex := calcPieceHashes(pieces)
	numOfPieces := len(hashByIndex)
	fmt.Println("number of pieces:", numOfPieces)

	infoHashBytes, err := getInfoHash(info)
	if err != nil {
		return err
	}

	responseBodyBytes, err := sendRequest(baseUrl, infoHashBytes, fileLength)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err.Error())
	}

	resp, _, err := decodeBencode(responseBodyBytes)
	if err != nil {
		return fmt.Errorf("error failed to decoded response: %s", err.Error())
	}

	respDict := resp.(map[string]any)
	peers := getPeers(respDict)
	if len(peers) < 1 {
		return fmt.Errorf("did not receive enough peers")
	}

	workers := make([]pieceDownloader, len(peers))
	fmt.Println("creating", len(peers), "workers")
	for i, p := range peers {
		workers[i] = pieceDownloader{
			peerConnectionString: p,
			infoHashBytes:        infoHashBytes,
			fileLength:           fileLength,
			pieceLength:          pieceLength,
			pieceHashesByIndex:   hashByIndex,
		}
	}

	// comms channels
	results := make(chan downloadedPiece, numOfPieces)
	outrightFailures := make(chan int, numOfPieces)
	piecesToDownload := make(chan pieceToDownload, numOfPieces)

	// start workers
	fmt.Println("starting workers...")
	var pieceDownloaderWaitGroup sync.WaitGroup
	cleanup := sync.OnceFunc(func() {
		close(results)
		close(outrightFailures)
	})
	for _, w := range workers {
		pieceDownloaderWaitGroup.Add(1)
		go func() {
			defer pieceDownloaderWaitGroup.Done()
			for downloadedablePiece := range piecesToDownload {
				fmt.Println("downloading piece index", downloadedablePiece.pieceIndex)
				pieceIndex := downloadedablePiece.pieceIndex
				pieceBytes, err := w.Download(pieceIndex)
				if err != nil {
					fmt.Printf("failed to download piece index %d, reinserting to queue: %s\n", pieceIndex, err.Error())
					if downloadedablePiece.attempt <= 10 {
						// time.Sleep(100 * time.Millisecond)
						piecesToDownload <- pieceToDownload{
							pieceIndex: pieceIndex,
							attempt:    downloadedablePiece.attempt + 1,
						}
					} else {
						outrightFailures <- pieceIndex
					}
				} else {
					fmt.Println("downloaded piece index", pieceIndex)
					results <- downloadedPiece{
						pieceIndex: pieceIndex,
						piece:      pieceBytes,
					}
				}
			}
			cleanup()
		}()
	}

	// seed queue
	fmt.Println("seeding pieces to download queue", len(hashByIndex))
	for pieceIndex := range hashByIndex {
		fmt.Println("seeding piece index", pieceIndex)
		piecesToDownload <- pieceToDownload{
			pieceIndex: pieceIndex,
			attempt:    1,
		}
	}

	// collect results from workers
	fmt.Println("collecting results from workers")
	downloadedFilePieces := make(map[int][]byte)
	failedFilePieces := make(map[int]any)
	for len(downloadedFilePieces)+len(failedFilePieces) < len(hashByIndex) {
		select {
		case dp := <-results:
			downloadedFilePieces[dp.pieceIndex] = dp.piece
		case fp := <-outrightFailures:
			failedFilePieces[fp] = nil
		}
	}

	// stop workers
	fmt.Println("waiting for workers to finish...")
	close(piecesToDownload)
	pieceDownloaderWaitGroup.Wait()
	fmt.Println("workers finished. Checking results...")

	// if any pice persistently failed, then fail
	if len(failedFilePieces) > 0 {
		return fmt.Errorf("failed to download one or more pieces: %d", len(failedFilePieces))
	}

	// collect pieces into file
	fmt.Println("all pieces downloaded and hashes checked, piecing together file")
	fileBytes := []byte{}
	for pieceIndex := 0; pieceIndex < numOfPieces; pieceIndex++ {
		piece, ok := downloadedFilePieces[pieceIndex]
		if !ok {
			return fmt.Errorf("missing download piece! %d", pieceIndex)
		}
		fileBytes = append(fileBytes, piece...)
	}

	// write file
	fmt.Println("writing file")
	if err = os.WriteFile(downloadTarget, fileBytes, 0666); err != nil {
		return fmt.Errorf("failed to open temp file to write file: %s", err.Error())
	}

	fmt.Println("successfully wrote file. Finished")
	return nil
}

type pieceDownloader struct {
	peerConnectionString string
	infoHashBytes        []byte
	fileLength           int
	pieceLength          int
	pieceHashesByIndex   map[int]string
}

type pieceToDownload struct {
	pieceIndex int
	attempt    int
}

type downloadedPiece struct {
	pieceIndex int
	piece      []byte
}

func (p pieceDownloader) Download(pieceIndex int) ([]byte, error) {
	peer := p.peerConnectionString
	hs := handshake{
		infoHash: p.infoHashBytes,
		peerID:   createRandomID(),
	}

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return nil, fmt.Errorf("failed to connect via tcp to peer: %s", err.Error())
	}
	defer conn.Close()

	_, err = doHandshakeOnConnection(conn, &hs)
	if err != nil {
		return nil, fmt.Errorf("failed to do handshake with peer: %s", err.Error())
	}

	response, err := waitForNextMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed wait for new message: %s", err.Error())
	}
	_, _ = parseMessage(response)

	response, err = sendInterested(conn)
	if err != nil {
		return nil, fmt.Errorf("failed send interested message: %s", err.Error())
	}
	_, _ = parseMessage(response)

	actualPieceLength := getPieceLengthForIndex(p.fileLength, p.pieceLength, pieceIndex)
	expectedBlocks := calcExpectedBlocks(actualPieceLength)

	currentOffset := 0
	blocks := [][]byte{}
	for i := 0; i < expectedBlocks; i++ {
		requestLength := int(math.Min(float64(sixteenKilobytes), float64(actualPieceLength-currentOffset)))
		message := createRequestMessage(pieceIndex, currentOffset, requestLength)

		_, err := conn.Write(message)
		if err != nil {
			return nil, fmt.Errorf("failed to read response after request message: %s", err.Error())
		}

		resp, err := readExactLength(conn, requestLength+13)
		if err != nil {
			return nil, fmt.Errorf("failed to read piece message: %s", err.Error())
		}

		_, _, _, block := parsePieceMessage(resp)
		blocks = append(blocks, block)
		currentOffset += requestLength
	}

	downloadedPiece := []byte{}
	for _, b := range blocks {
		downloadedPiece = append(downloadedPiece, b...)
	}

	pieceHash, err := hashBytesNew(downloadedPiece)
	if err != nil {
		return nil, fmt.Errorf("failed to generate hash for new piece")
	}

	expectedHash := p.pieceHashesByIndex[pieceIndex]
	fmt.Println(pieceIndex, ":", time.Now().Format(time.RFC3339), "eHash", expectedHash)
	fmt.Println(pieceIndex, ":", time.Now().Format(time.RFC3339), "aHash", pieceHash)
	if pieceHash != expectedHash {
		return nil, fmt.Errorf("piece hash did not match hash in torrent file. actual: %s, expected: %s", pieceHash, expectedHash)
	}
	return downloadedPiece, nil
}

func magnet_parse(link string) error {
	data, err := parseMagnetLink(link)
	if err != nil {
		return err
	}
	fmt.Printf("Tracker URL: %s\n", data.trackerURL)
	fmt.Printf("Info Hash: %s\n", data.infoHash)
	return nil
}

func parseMagnetLink(link string) (*magnetLinkData, error) {
	magnetUrl, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("failed to parse magnet url: %s", err.Error())
	}

	query := magnetUrl.Query()
	xt := query.Get("xt")
	dn := query.Get("dn")
	tr := query.Get("tr")

	if !strings.HasPrefix(xt, "urn:btih:") {
		return nil, fmt.Errorf("unexpected magnet url type. Missing urn:bith")
	}

	infoHash := xt[9:]

	return &magnetLinkData{
		trackerURL: tr,
		fileName:   dn,
		infoHash:   infoHash,
	}, nil
}

type magnetLinkData struct {
	trackerURL string
	fileName   string
	infoHash   string
}

func magnet_handshake(link string) error {
	data, err := parseMagnetLink(link)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %s", err.Error())
	}

	infoHashBytes, err := hex.DecodeString(data.infoHash)
	if err != nil {
		return fmt.Errorf("failed to decode info hash: %s", err.Error())
	}

	responseBodyBytes, err := sendRequest(data.trackerURL, infoHashBytes, 999)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err.Error())
	}

	resp, _, err := decodeBencode(responseBodyBytes)
	if err != nil {
		return fmt.Errorf("error failed to decoded response: %s", err.Error())
	}

	respDict := resp.(map[string]any)
	peers := getPeers(respDict)
	if len(peers) < 1 {
		return fmt.Errorf("did not receive enough peers")
	}

	havePrinted := false
	for _, peer := range peers {
		// fmt.Println("starting with peer:", peer)
		hs := handshake{
			infoHash:          infoHashBytes,
			peerID:            createRandomID(),
			supportExtensions: true,
		}

		conn, err := net.Dial("tcp", peer)
		if err != nil {
			return fmt.Errorf("failed to connect via tcp to peer: %s", err.Error())
		}
		defer conn.Close()

		handshakeResponse, err := doHandshakeOnConnection(conn, &hs)
		if err != nil {
			return fmt.Errorf("failed to do handshake with peer: %s", err.Error())
		}

		if !havePrinted {
			fmt.Printf("Peer ID: %s\n", hex.EncodeToString(handshakeResponse.peerID))
			havePrinted = true
		}

		if !handshakeResponse.supportExtensions {
			fmt.Printf("peer indicated that it does not support extensions. Try next peer...\n")
			continue
		}
		// fmt.Println("peer supports extensions")

		fmt.Println("reading bitfield message")
		response, err := readOneResponse(conn)
		if err != nil {
			return fmt.Errorf("failed wait for bitfield message: %s", err.Error())
		}

		id, _ := parseMessage(response)
		fmt.Println("received message with id:", id)

		fmt.Println("waiting for extension handshake?")
		extensionHandshakeMessage, err := readOneResponse(conn)
		if err != nil {
			return fmt.Errorf("failed to read extension handshake")
		}

		payload := extractPayloadFromExtensionHandshakeMessage(extensionHandshakeMessage)
		decodedPayload, _, err := decodeBencode(payload)
		if err != nil {
			return fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		jsonBytes, err := json.Marshal(decodedPayload)
		fmt.Println("extension payload:", string(jsonBytes))

		m := decodedPayload.(map[string]any)["m"]
		utMetadata := m.(map[string]any)["ut_metadata"].(int)

		fmt.Println("Peer Metadata Extension ID:", utMetadata)

		message, err := createExtensionMessage()
		if err != nil {
			return fmt.Errorf("failed to create extension message: %s", err.Error())
		}

		_, err = conn.Write(message)
		if err != nil {
			return fmt.Errorf("failed to write interested message")
		}

		// fmt.Println(string(extendedHandshakeResponse))

	}
	return nil
}

func createExtensionMessage() ([]byte, error) {
	length := uint32(2) // includes id, and extension message id

	payload := map[string]any{
		"m": map[string]any{
			"ut_metadata": ourMetadataExtensionId,
		},
	}
	encoded, err := encodeBencode(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to bencode extension payload: %s", err.Error())
	}

	// fmt.Println(string(encoded))

	length += uint32(len(encoded))

	message := []byte{}
	message = binary.BigEndian.AppendUint32(message, length)
	message = append(message, byte(20))
	message = append(message, byte(0))
	message = append(message, encoded...)
	// fmt.Println(message)
	return message, nil
}

func createRequestMetadataMessage(id int) ([]byte, error) {
	length := uint32(2) // peer message id and metadata id from peer

	requestMessage := map[string]any{
		"msg_type": 0,
		"piece":    0,
	}

	encoded, err := encodeBencode(requestMessage)
	if err != nil {
		return nil, fmt.Errorf("failed to encoded request message: %s", err.Error())
	}

	length += uint32(len(encoded))

	message := []byte{}
	message = binary.BigEndian.AppendUint32(message, length)
	message = append(message, byte(20)) // extension message
	message = append(message, byte(id)) // metadata id from peer
	message = append(message, encoded...)

	return message, nil
}

func sendMessageAndReadExactResponse(conn net.Conn, message []byte) ([]byte, error) {
	_, err := conn.Write(message)
	if err != nil {
		return nil, fmt.Errorf("failed to write interested message")
	}

	return readOneResponse(conn)
}

func readOneResponse(conn net.Conn) ([]byte, error) {
	buffer := make([]byte, 4)
	n, err := conn.Read(buffer)
	if err != nil {
		return nil, fmt.Errorf("failed to read length from tcp connection: %s", err.Error())
	}

	if n != 4 {
		return nil, fmt.Errorf("failed to read message length from connection: %s", err.Error())
	}

	fmt.Println(buffer)

	length := binary.BigEndian.Uint32(buffer)
	fmt.Println("need to read", length, "bytes for message")

	result, err := readExactLength(conn, int(length))
	if err != nil {
		return nil, fmt.Errorf("failed to read length given at beginning of message: %s", err.Error())
	}

	return append(buffer, result...), nil
}

func extractPayloadFromExtensionHandshakeMessage(message []byte) []byte {
	// message len = 4 bytes + message id = 1 byte + extension message id = 1 byte
	preambleLength := 4 + 1 + 1
	return message[preambleLength:]
}

func magnet_info(link string) error {
	data, err := parseMagnetLink(link)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %s", err.Error())
	}

	infoHashBytes, err := hex.DecodeString(data.infoHash)
	if err != nil {
		return fmt.Errorf("failed to decode info hash: %s", err.Error())
	}

	responseBodyBytes, err := sendRequest(data.trackerURL, infoHashBytes, 999)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err.Error())
	}

	resp, _, err := decodeBencode(responseBodyBytes)
	if err != nil {
		return fmt.Errorf("error failed to decoded response: %s", err.Error())
	}

	respDict := resp.(map[string]any)
	peers := getPeers(respDict)
	if len(peers) < 1 {
		return fmt.Errorf("did not receive enough peers")
	}

	havePrinted := false
	for _, peer := range peers {
		hs := handshake{
			infoHash:          infoHashBytes,
			peerID:            createRandomID(),
			supportExtensions: true,
		}

		conn, err := net.Dial("tcp", peer)
		if err != nil {
			return fmt.Errorf("failed to connect via tcp to peer: %s", err.Error())
		}
		defer conn.Close()

		handshakeResponse, err := doHandshakeOnConnection(conn, &hs)
		if err != nil {
			return fmt.Errorf("failed to do handshake with peer: %s", err.Error())
		}

		if !havePrinted {
			fmt.Printf("Peer ID: %s\n", hex.EncodeToString(handshakeResponse.peerID))
			havePrinted = true
		}

		if !handshakeResponse.supportExtensions {
			fmt.Printf("peer indicated that it does not support extensions. Try next peer...\n")
			continue
		}

		response, err := readOneResponse(conn)
		if err != nil {
			return fmt.Errorf("failed wait for bitfield message: %s", err.Error())
		}

		parseMessage(response)

		extensionHandshakeMessage, err := readOneResponse(conn)
		if err != nil {
			return fmt.Errorf("failed to read extension handshake")
		}

		payload := extractPayloadFromExtensionHandshakeMessage(extensionHandshakeMessage)
		decodedPayload, _, err := decodeBencode(payload)
		if err != nil {
			return fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		m := decodedPayload.(map[string]any)["m"]
		utMetadata := m.(map[string]any)["ut_metadata"].(int)

		message, err := createExtensionMessage()
		if err != nil {
			return fmt.Errorf("failed to create extension message: %s", err.Error())
		}

		_, err = conn.Write(message)
		if err != nil {
			return fmt.Errorf("failed to write interested message")
		}

		requestMetaMessage, err := createRequestMetadataMessage(utMetadata)
		if err != nil {
			return fmt.Errorf("failed to create request metadata message: %s", err.Error())
		}

		_, err = conn.Write(requestMetaMessage)
		if err != nil {
			return fmt.Errorf("failed to write create metadata message to connection: %s", err.Error())
		}

		response, err = readOneResponse(conn)
		if err != nil {
			return fmt.Errorf("failed to read extension message from connection: %s", err.Error())
		}

		payload = extractPayloadFromExtensionHandshakeMessage(response)
		decodedPayload, index, err := decodeBencode(payload)
		if err != nil {
			return fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		metaDataPieceContents, _, err := decodeBencode(payload[index:])
		if err != nil {
			return fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		h := sha1.New()
		h.Write(payload[index:])
		hash := hex.EncodeToString(h.Sum(nil))

		pieces := metaDataPieceContents.(map[string]any)["pieces"].(string)
		hashes := []string{}
		for cur := 0; cur < len(pieces); cur += 20 {
			hashes = append(hashes, hex.EncodeToString([]byte(pieces[cur:cur+20])))
		}

		fmt.Println("Tracker URL:", data.trackerURL)
		fmt.Println("Length:", metaDataPieceContents.(map[string]any)["length"].(int))
		fmt.Println("Info Hash:", hash)
		fmt.Println("Piece Length:", metaDataPieceContents.(map[string]any)["piece length"].(int))
		fmt.Println("Piece Hashes:")
		for _, h := range hashes {
			fmt.Println(h)
		}
		break
	}
	return nil
}

func magnet_download_piece(target, link string, pieceIndex int) error {
	data, err := parseMagnetLink(link)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %s", err.Error())
	}

	infoHashBytes, err := hex.DecodeString(data.infoHash)
	if err != nil {
		return fmt.Errorf("failed to decode info hash: %s", err.Error())
	}

	responseBodyBytes, err := sendRequest(data.trackerURL, infoHashBytes, 999)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err.Error())
	}

	resp, _, err := decodeBencode(responseBodyBytes)
	if err != nil {
		return fmt.Errorf("error failed to decoded response: %s", err.Error())
	}

	respDict := resp.(map[string]any)
	peers := getPeers(respDict)
	if len(peers) < 1 {
		return fmt.Errorf("did not receive enough peers")
	}

	var pd pieceDownloader = pieceDownloader{}
	for _, peer := range peers {
		hs := handshake{
			infoHash:          infoHashBytes,
			peerID:            createRandomID(),
			supportExtensions: true,
		}

		conn, err := net.Dial("tcp", peer)
		if err != nil {
			return fmt.Errorf("failed to connect via tcp to peer: %s", err.Error())
		}
		defer conn.Close()

		handshakeResponse, err := doHandshakeOnConnection(conn, &hs)
		if err != nil {
			return fmt.Errorf("failed to do handshake with peer: %s", err.Error())
		}

		if !handshakeResponse.supportExtensions {
			fmt.Printf("peer indicated that it does not support extensions. Try next peer...\n")
			continue
		}

		response, err := readOneResponse(conn)
		if err != nil {
			return fmt.Errorf("failed wait for bitfield message: %s", err.Error())
		}

		parseMessage(response)

		extensionHandshakeMessage, err := readOneResponse(conn)
		if err != nil {
			return fmt.Errorf("failed to read extension handshake")
		}

		payload := extractPayloadFromExtensionHandshakeMessage(extensionHandshakeMessage)
		decodedPayload, _, err := decodeBencode(payload)
		if err != nil {
			return fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		m := decodedPayload.(map[string]any)["m"]
		utMetadata := m.(map[string]any)["ut_metadata"].(int)

		message, err := createExtensionMessage()
		if err != nil {
			return fmt.Errorf("failed to create extension message: %s", err.Error())
		}

		_, err = conn.Write(message)
		if err != nil {
			return fmt.Errorf("failed to write interested message")
		}

		requestMetaMessage, err := createRequestMetadataMessage(utMetadata)
		if err != nil {
			return fmt.Errorf("failed to create request metadata message: %s", err.Error())
		}

		_, err = conn.Write(requestMetaMessage)
		if err != nil {
			return fmt.Errorf("failed to write create metadata message to connection: %s", err.Error())
		}

		response, err = readOneResponse(conn)
		if err != nil {
			return fmt.Errorf("failed to read extension message from connection: %s", err.Error())
		}

		payload = extractPayloadFromExtensionHandshakeMessage(response)
		decodedPayload, index, err := decodeBencode(payload)
		if err != nil {
			return fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		metaDataPieceContents, _, err := decodeBencode(payload[index:])
		if err != nil {
			return fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		// h := sha1.New()
		// h.Write(payload[index:])
		// checkInfoHashBytes := h.Sum(nil)
		// if checkInfoHashBytes

		pieces := metaDataPieceContents.(map[string]any)["pieces"].(string)
		pieceHashesByIndex := calcPieceHashes(pieces)

		pd = pieceDownloader{
			peerConnectionString: peer,
			infoHashBytes:        infoHashBytes,
			fileLength:           metaDataPieceContents.(map[string]any)["length"].(int),
			pieceLength:          metaDataPieceContents.(map[string]any)["piece length"].(int),
			pieceHashesByIndex:   pieceHashesByIndex,
		}

		break
	}

	piece, err := pd.Download(pieceIndex)
	if err != nil {
		fmt.Errorf("failed to download piece from peer: %s", err.Error())
	}

	fmt.Println("writing file")
	if err = os.WriteFile(target, piece, 0666); err != nil {
		return fmt.Errorf("failed to open temp file to write file: %s", err.Error())
	}

	return nil
}

type downloadInfo struct {
	infoHashBytes      []byte
	fileLength         int
	pieceLength        int
	pieceHashesByIndex map[int]string
}

func getDownloadInfoThroughMetadataFromPeers(peers []string, infoHashBytes []byte) (downloadInfo, error) {
	numOfPiecesInFile := 0
	var di *downloadInfo = nil
	for _, peer := range peers {
		hs := handshake{
			infoHash:          infoHashBytes,
			peerID:            createRandomID(),
			supportExtensions: true,
		}

		conn, err := net.Dial("tcp", peer)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to connect via tcp to peer: %s", err.Error())
		}
		defer conn.Close()

		handshakeResponse, err := doHandshakeOnConnection(conn, &hs)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to do handshake with peer: %s", err.Error())
		}

		if !handshakeResponse.supportExtensions {
			fmt.Printf("peer indicated that it does not support extensions. Try next peer...\n")
			continue
		}

		response, err := readOneResponse(conn)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed wait for bitfield message: %s", err.Error())
		}

		parseMessage(response)

		extensionHandshakeMessage, err := readOneResponse(conn)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to read extension handshake")
		}

		payload := extractPayloadFromExtensionHandshakeMessage(extensionHandshakeMessage)
		decodedPayload, _, err := decodeBencode(payload)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		m := decodedPayload.(map[string]any)["m"]
		utMetadata := m.(map[string]any)["ut_metadata"].(int)

		message, err := createExtensionMessage()
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to create extension message: %s", err.Error())
		}

		_, err = conn.Write(message)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to write interested message")
		}

		requestMetaMessage, err := createRequestMetadataMessage(utMetadata)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to create request metadata message: %s", err.Error())
		}

		_, err = conn.Write(requestMetaMessage)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to write create metadata message to connection: %s", err.Error())
		}

		response, err = readOneResponse(conn)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to read extension message from connection: %s", err.Error())
		}

		payload = extractPayloadFromExtensionHandshakeMessage(response)
		decodedPayload, index, err := decodeBencode(payload)
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		metaDataPieceContents, _, err := decodeBencode(payload[index:])
		if err != nil {
			return downloadInfo{}, fmt.Errorf("failed to decode payload: %s", err.Error())
		}

		// we could verify the hash and fail here but not _really_ necessary right now, so leaving it.
		// h := sha1.New()
		// h.Write(payload[index:])
		// checkInfoHashBytes := h.Sum(nil)
		// if checkInfoHashBytes

		pieces := metaDataPieceContents.(map[string]any)["pieces"].(string)
		pieceHashesByIndex := calcPieceHashes(pieces)

		fileLength := metaDataPieceContents.(map[string]any)["length"].(int)
		pieceLength := metaDataPieceContents.(map[string]any)["piece length"].(int)
		numOfPiecesInFile = fileLength / pieceLength
		if fileLength%pieceLength > 0 {
			numOfPiecesInFile++
		}
		fmt.Println("num of pieces in file:", numOfPiecesInFile)

		if di == nil {
			di = &downloadInfo{
				infoHashBytes:      infoHashBytes,
				fileLength:         fileLength,
				pieceLength:        pieceLength,
				pieceHashesByIndex: pieceHashesByIndex,
			}
		} else {
			for index, hash := range pieceHashesByIndex {
				if _, ok := di.pieceHashesByIndex[index]; !ok {
					di.pieceHashesByIndex[index] = hash
				}
			}
		}

		fmt.Println("current number of piece hashes by index:", len(di.pieceHashesByIndex))

		if len(di.pieceHashesByIndex) == numOfPiecesInFile {
			return *di, nil
		}
	}
	return downloadInfo{}, fmt.Errorf("failed to find enough peers that supports extensions")
}

func magnet_download(target, link string) error {
	data, err := parseMagnetLink(link)
	if err != nil {
		return fmt.Errorf("failed to parse magnet link: %s", err.Error())
	}

	infoHashBytes, err := hex.DecodeString(data.infoHash)
	if err != nil {
		return fmt.Errorf("failed to decode info hash: %s", err.Error())
	}

	responseBodyBytes, err := sendRequest(data.trackerURL, infoHashBytes, 999)
	if err != nil {
		return fmt.Errorf("error reading response body: %s", err.Error())
	}

	resp, _, err := decodeBencode(responseBodyBytes)
	if err != nil {
		return fmt.Errorf("error failed to decoded response: %s", err.Error())
	}

	respDict := resp.(map[string]any)
	peers := getPeers(respDict)
	if len(peers) < 1 {
		return fmt.Errorf("did not receive enough peers")
	}

	downloadInfo, err := getDownloadInfoThroughMetadataFromPeers(peers, infoHashBytes)
	if err != nil {
		return fmt.Errorf("failed to get download info from extension supporting peers: %s", err.Error())
	}

	fmt.Printf("%+v", downloadInfo)

	if err = downloadFileUsingWorkers(target, peers, downloadInfo); err != nil {
		return fmt.Errorf("failed to download the file using workers: %s", err.Error())
	}

	return nil
}

func downloadFileUsingWorkers(downloadTarget string, peers []string, di downloadInfo) error {
	numOfPieces := len(di.pieceHashesByIndex)
	fmt.Println("number of pieces:", numOfPieces)

	workers := make([]pieceDownloader, len(peers))
	fmt.Println("creating", len(peers), "workers")
	for i, p := range peers {
		workers[i] = pieceDownloader{
			peerConnectionString: p,
			infoHashBytes:        di.infoHashBytes,
			fileLength:           di.fileLength,
			pieceLength:          di.pieceLength,
			pieceHashesByIndex:   di.pieceHashesByIndex,
		}
	}

	// comms channels
	results := make(chan downloadedPiece, numOfPieces)
	outrightFailures := make(chan int, numOfPieces)
	piecesToDownload := make(chan pieceToDownload, numOfPieces)

	// start workers
	fmt.Println("starting workers...")
	var pieceDownloaderWaitGroup sync.WaitGroup
	cleanup := sync.OnceFunc(func() {
		close(results)
		close(outrightFailures)
	})
	for _, w := range workers {
		pieceDownloaderWaitGroup.Add(1)
		go func() {
			defer pieceDownloaderWaitGroup.Done()
			for downloadedablePiece := range piecesToDownload {
				fmt.Println("downloading piece index", downloadedablePiece.pieceIndex)
				pieceIndex := downloadedablePiece.pieceIndex
				pieceBytes, err := w.Download(pieceIndex)
				if err != nil {
					fmt.Printf("failed to download piece index %d, reinserting to queue: %s\n", pieceIndex, err.Error())
					if downloadedablePiece.attempt <= 10 {
						// time.Sleep(100 * time.Millisecond)
						piecesToDownload <- pieceToDownload{
							pieceIndex: pieceIndex,
							attempt:    downloadedablePiece.attempt + 1,
						}
					} else {
						outrightFailures <- pieceIndex
					}
				} else {
					fmt.Println("downloaded piece index", pieceIndex)
					results <- downloadedPiece{
						pieceIndex: pieceIndex,
						piece:      pieceBytes,
					}
				}
			}
			cleanup()
		}()
	}

	// seed queue
	fmt.Println("seeding pieces to download queue", len(di.pieceHashesByIndex))
	for pieceIndex := range di.pieceHashesByIndex {
		fmt.Println("seeding piece index", pieceIndex)
		piecesToDownload <- pieceToDownload{
			pieceIndex: pieceIndex,
			attempt:    1,
		}
	}

	// collect results from workers
	fmt.Println("collecting results from workers")
	downloadedFilePieces := make(map[int][]byte)
	failedFilePieces := make(map[int]any)
	for len(downloadedFilePieces)+len(failedFilePieces) < len(di.pieceHashesByIndex) {
		select {
		case dp := <-results:
			downloadedFilePieces[dp.pieceIndex] = dp.piece
		case fp := <-outrightFailures:
			failedFilePieces[fp] = nil
		}
	}

	// stop workers
	fmt.Println("waiting for workers to finish...")
	close(piecesToDownload)
	pieceDownloaderWaitGroup.Wait()
	fmt.Println("workers finished. Checking results...")

	// if any pice persistently failed, then fail
	if len(failedFilePieces) > 0 {
		return fmt.Errorf("failed to download one or more pieces: %d", len(failedFilePieces))
	}

	// collect pieces into file
	fmt.Println("all pieces downloaded and hashes checked, piecing together file")
	fileBytes := []byte{}
	for pieceIndex := 0; pieceIndex < numOfPieces; pieceIndex++ {
		piece, ok := downloadedFilePieces[pieceIndex]
		if !ok {
			return fmt.Errorf("missing download piece! %d", pieceIndex)
		}
		fileBytes = append(fileBytes, piece...)
	}

	// write file
	fmt.Println("writing file")
	if err := os.WriteFile(downloadTarget, fileBytes, 0666); err != nil {
		return fmt.Errorf("failed to open temp file to write file: %s", err.Error())
	}

	return nil
}
