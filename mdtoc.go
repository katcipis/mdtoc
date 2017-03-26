package mdtoc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
)

const headerFormat = "- [%s](#%s)"
const atxHeader = "#"
const headerIdent = "    "

func isValidHeaderRune(r rune) bool {
	return unicode.IsNumber(r) || unicode.IsLetter(r) || unicode.IsSpace(r)
}

func normalizeHeader(header string) string {
	lowerNoHash := strings.TrimLeft(strings.ToLower(header), "#")
	noInvalidChars := []rune{}

	for _, r := range lowerNoHash {
		if isValidHeaderRune(r) {
			noInvalidChars = append(noInvalidChars, r)
		}
	}

	return strings.Replace(string(noInvalidChars), " ", "-", -1)
}

type writer func(n []byte)

func writeHeader(
	writeOutput writer,
	level int,
	header string,
	headersCount map[string]int,
) {
	normalizedHeader := normalizeHeader(header)
	count := headersCount[normalizedHeader]
	headersCount[normalizedHeader] = count + 1
	if count > 0 {
		normalizedHeader = fmt.Sprintf("%s-%d", normalizedHeader, count)
	}
	line := fmt.Sprintf(
		headerFormat,
		header,
		normalizedHeader,
	)
	for i := 1; i < level; i++ {
		writeOutput([]byte(headerIdent))
	}
	writeOutput([]byte(line + "\n"))
}

func parseHeader(line string) (int, string, bool) {
	if !startsWithAtxHeader(line) {
		return 0, "", false
	}
	spaceTrimmed := strings.TrimRight(line, " ")
	parsed := strings.Split(spaceTrimmed, " ")
	if len(parsed) == 1 {
		return 0, "", false
	}
	headerlevel := len(parsed[0])
	header := parsed[1:]
	return headerlevel, strings.Join(header, " "), true
}

func startsWithAtxHeader(line string) bool {
	return strings.Index(line, atxHeader) == 0
}

func skipUntil(scanner *bufio.Scanner, stop func(string) bool) error {
	for scanner.Scan() {
		if stop(scanner.Text()) {
			return nil
		}
	}
	return errors.New("skipped all content")
}

func Generate(input io.Reader, output io.Writer) error {
	headerStart := "<!-- mdtocstart -->"
	tocHeader := "# Table of Contents"
	headerEnd := "<!-- mdtocend -->"
	scanner := bufio.NewScanner(input)
	headersCount := map[string]int{}

	var writeErr error
	writeOutput := func(b []byte) {
		if writeErr != nil {
			return
		}
		_, writeErr = output.Write(b)
	}

	var original bytes.Buffer
	var wroteHeader bool

	for scanner.Scan() {
		line := scanner.Text()
		if line == headerStart {
			err := skipUntil(scanner, func(l string) bool { return l == headerEnd })
			if err != nil {
				return fmt.Errorf("error removing headers(corrupted headers?): %s", err)
			}
			skipUntil(scanner, func(l string) bool { return l != "" })
			line = scanner.Text()
		}
		_, err := original.Write([]byte(line + "\n"))
		if err != nil {
			return err
		}
		level, header, ok := parseHeader(line)
		if !ok {
			continue
		}
		if !wroteHeader {
			writeOutput([]byte(headerStart))
			writeOutput([]byte("\n"))
			writeOutput([]byte(tocHeader))
			writeOutput([]byte("\n\n"))
			wroteHeader = true
		}
		writeHeader(writeOutput, level, header, headersCount)
	}

	if scanner.Err() != nil {
		return scanner.Err()
	}

	if wroteHeader {
		writeOutput([]byte(headerEnd))
		writeOutput([]byte("\n\n"))
	}

	writeOutput(original.Bytes())
	return writeErr
}

func GenerateFromFile(inputpath string, output io.Writer) error {
	file, err := os.Open(inputpath)
	defer file.Close()
	if err != nil {
		return fmt.Errorf("GenerateFromFile: error opening file: %s", err)
	}
	return Generate(file, output)
}

func GenerateInPlace(inputpath string) error {
	var output bytes.Buffer
	err := GenerateFromFile(inputpath, &output)
	if err != nil {
		return err
	}

	file, err := os.Create(inputpath)
	defer file.Close()
	if err != nil {
		// TODO: That is why we need a backup file for the original one :-)
		return fmt.Errorf("GenerateInPlace: unable to truncate file: %s", err)
	}

	expectedwrite := int64(output.Len())
	written, err := io.Copy(file, &output)
	if err != nil {
		// TODO: That is why we need a backup file for the original one :-)
		return fmt.Errorf("GenerateInPlace: unable to copy contents: %s", err)
	}
	if written != expectedwrite {
		// TODO: That is why we need a backup file for the original one :-)
		return fmt.Errorf(
			"GenerateInPlace: unable to copy contents: wrote %d expected %d",
			written,
			expectedwrite,
		)
	}
	return nil
}
