package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	sourceURL = "https://raw.githubusercontent.com/danielmiessler/SecLists/" +
		"d3cbcbfe5120ee735dd783e477836619debdc57c/" +
		"Passwords/Common-Credentials/xato-net-10-million-passwords-100000.txt"
	sourceSHA256      = "1472aafa2561df5e3293aee252aee3ca660c12b399a283cf808bb01b39be388b"
	outputFile        = "common-passwords.bloom"
	falsePositiveRate = 0.000001
)

func main() {
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(sourceURL)
	if err != nil {
		fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fatal(fmt.Errorf("download: %s", response.Status))
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		fatal(err)
	}
	sum := sha256.Sum256(content)
	if hex.EncodeToString(sum[:]) != sourceSHA256 {
		fatal(fmt.Errorf("source checksum mismatch: %x", sum))
	}

	var values []string
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		value := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if value != "" {
			values = append(values, value)
		}
	}
	if err := scanner.Err(); err != nil {
		fatal(err)
	}
	size := uint64(math.Ceil(
		-float64(len(values)) * math.Log(falsePositiveRate) /
			(math.Ln2 * math.Ln2),
	))
	keys := uint32(math.Round(float64(size) / float64(len(values)) * math.Ln2))
	bits := make([]byte, (size+7)/8)
	for _, value := range values {
		digest := sha256.Sum256([]byte(value))
		first := binary.BigEndian.Uint64(digest[:8])
		second := binary.BigEndian.Uint64(digest[8:16])
		if second == 0 {
			second = 0x9e3779b97f4a7c15
		}
		for index := uint32(0); index < keys; index++ {
			position := (first + uint64(index)*second) % size
			bits[position/8] |= 1 << (position % 8)
		}
	}
	output := make([]byte, 19+len(bits))
	copy(output[:7], "YLPWBL1")
	binary.BigEndian.PutUint64(output[7:15], size)
	binary.BigEndian.PutUint32(output[15:19], keys)
	copy(output[19:], bits)
	if err := os.WriteFile(outputFile, output, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("generated %s: entries=%d bits=%d hashes=%d bytes=%d\n",
		outputFile, len(values), size, keys, len(output))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
