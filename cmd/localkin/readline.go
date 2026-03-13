package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/term"
)

// ─── Command History ──────────────────────────────────────

var (
	cmdHistory    []string // in-memory history
	cmdHistFile   string   // persistence path
	cmdHistoryMax = 500    // max entries kept
)

// InitHistory loads command history from disk. Call once before REPL starts.
func InitHistory(path string) {
	cmdHistFile = path
	cmdHistory = loadHistoryFile(path)
}

func loadHistoryFile(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.TrimRight(l, "\r")
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func appendHistory(line string) {
	if line == "" {
		return
	}
	if len(cmdHistory) > 0 && cmdHistory[len(cmdHistory)-1] == line {
		return
	}
	cmdHistory = append(cmdHistory, line)
	if len(cmdHistory) > cmdHistoryMax {
		cmdHistory = cmdHistory[len(cmdHistory)-cmdHistoryMax:]
	}
	saveHistoryFile()
}

func saveHistoryFile() {
	if cmdHistFile == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(cmdHistFile), 0755)
	_ = os.WriteFile(cmdHistFile, []byte(strings.Join(cmdHistory, "\n")+"\n"), 0600)
}

// readLine reads a line of input with UTF-8 support and command history.
// Up/Down arrows navigate history. Falls back to bufio.Scanner for non-terminals.
func readLine(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())

	if !term.IsTerminal(fd) {
		fmt.Fprint(os.Stderr, prompt)
		s := bufio.NewScanner(os.Stdin)
		if s.Scan() {
			return s.Text(), nil
		}
		return "", io.EOF
	}

	old, err := term.MakeRaw(fd)
	if err != nil {
		fmt.Fprint(os.Stderr, prompt)
		s := bufio.NewScanner(os.Stdin)
		if s.Scan() {
			return s.Text(), nil
		}
		return "", io.EOF
	}
	defer term.Restore(fd, old)

	writeStr(prompt)

	var line []rune
	buf := make([]byte, 4)
	histIdx := -1
	var savedInput []rune

	for {
		n, err := os.Stdin.Read(buf[:1])
		if n == 0 || err != nil {
			writeStr("\r\n")
			return "", io.EOF
		}

		b := buf[0]

		switch {
		case b == '\r' || b == '\n': // Enter
			writeStr("\r\n")
			result := string(line)
			appendHistory(result)
			return result, nil

		case b == 3: // Ctrl+C
			writeStr("^C\r\n")
			return "", nil

		case b == 4 && len(line) == 0: // Ctrl+D on empty
			writeStr("\r\n")
			return "", io.EOF

		case b == 21: // Ctrl+U — kill line
			eraseRunes(line)
			line = nil

		case b == 12: // Ctrl+L — clear screen
			writeStr("\033[2J\033[H")
			writeStr(prompt + string(line))

		case b == 127 || b == 8: // Backspace / Ctrl+H
			if len(line) > 0 {
				r := line[len(line)-1]
				line = line[:len(line)-1]
				w := runeDisplayWidth(r)
				for i := 0; i < w; i++ {
					writeStr("\b \b")
				}
			}

		case b == 27: // Escape sequence (arrow keys)
			os.Stdin.Read(buf[:1])
			if buf[0] == '[' {
				os.Stdin.Read(buf[:1])
				code := buf[0]
				// Consume extended sequences like ESC [ 1 ; 5 C
				if code >= '0' && code <= '9' {
					for {
						os.Stdin.Read(buf[:1])
						if buf[0] == '~' || (buf[0] >= 'A' && buf[0] <= 'Z') {
							break
						}
					}
				}
				switch code {
				case 'A': // Up — older history
					if len(cmdHistory) == 0 {
						break
					}
					if histIdx == -1 {
						savedInput = make([]rune, len(line))
						copy(savedInput, line)
						histIdx = len(cmdHistory) - 1
					} else if histIdx > 0 {
						histIdx--
					} else {
						break
					}
					newLine := []rune(cmdHistory[histIdx])
					eraseRunes(line)
					line = newLine
					writeStr(string(line))

				case 'B': // Down — newer history
					if histIdx == -1 {
						break
					}
					if histIdx < len(cmdHistory)-1 {
						histIdx++
						newLine := []rune(cmdHistory[histIdx])
						eraseRunes(line)
						line = newLine
						writeStr(string(line))
					} else {
						histIdx = -1
						eraseRunes(line)
						line = savedInput
						savedInput = nil
						writeStr(string(line))
					}
				}
			}

		case b >= 0x20 && b < 0x7F: // ASCII printable
			line = append(line, rune(b))
			os.Stdout.Write([]byte{b})

		default: // Multi-byte UTF-8
			if b < 0x80 {
				continue
			}
			size := utf8ByteLen(b)
			buf[0] = b
			if size > 1 {
				if _, err := io.ReadFull(os.Stdin, buf[1:size]); err != nil {
					continue
				}
			}
			if r, _ := utf8.DecodeRune(buf[:size]); r != utf8.RuneError {
				line = append(line, r)
				os.Stdout.Write(buf[:size])
			}
		}
	}
}

func writeStr(s string) { os.Stdout.WriteString(s) }

func eraseRunes(runes []rune) {
	for i := len(runes) - 1; i >= 0; i-- {
		w := runeDisplayWidth(runes[i])
		for j := 0; j < w; j++ {
			writeStr("\b \b")
		}
	}
}

func runeDisplayWidth(r rune) int {
	if r >= 0x1100 && (r <= 0x115F ||
		(r >= 0x2E80 && r <= 0x9FFF) ||
		(r >= 0xAC00 && r <= 0xD7A3) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE10 && r <= 0xFE6F) ||
		(r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) ||
		(r >= 0x20000 && r <= 0x3FFFF)) {
		return 2
	}
	return 1
}

func utf8ByteLen(b byte) int {
	if b < 0xC0 {
		return 1
	}
	if b < 0xE0 {
		return 2
	}
	if b < 0xF0 {
		return 3
	}
	return 4
}
