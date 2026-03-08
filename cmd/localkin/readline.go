package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"unicode/utf8"

	"golang.org/x/term"
)

func readLine(prompt string) (string, error) {
	fd := int(os.Stdin.Fd())
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
	os.Stdout.WriteString(prompt)
	var line []rune
	buf := make([]byte, 4)
	for {
		if _, err := os.Stdin.Read(buf[:1]); err != nil {
			return string(line), err
		}
		switch b := buf[0]; {
		case b == '\r' || b == '\n':
			os.Stdout.WriteString("\r\n")
			return string(line), nil
		case b == 3: // Ctrl+C
			os.Stdout.WriteString("^C\r\n")
			return "", nil
		case b == 4 && len(line) == 0: // Ctrl+D on empty line
			return "", io.EOF
		case b == 127 || b == 8 || b == 21: // Backspace / Ctrl+H / Ctrl+U
			n := 1
			if b == 21 {
				n = len(line)
			}
			for ; n > 0 && len(line) > 0; n-- {
				for i := runeWidth(line[len(line)-1]); i > 0; i-- {
					os.Stdout.WriteString("\b \b")
				}
				line = line[:len(line)-1]
			}
		case b == 27: // ESC sequence (arrow keys etc.)
			os.Stdin.Read(buf[:2])
		case b < 32: // other control chars
		default:
			size := 1
			if b >= 0xC0 {
				size = 2
			}
			if b >= 0xE0 {
				size = 3
			}
			if b >= 0xF0 {
				size = 4
			}
			buf[0] = b
			if size > 1 {
				io.ReadFull(os.Stdin, buf[1:size])
			}
			if r, _ := utf8.DecodeRune(buf[:size]); r != utf8.RuneError {
				line = append(line, r)
				os.Stdout.Write(buf[:size])
			}
		}
	}
}

func runeWidth(r rune) int {
	if r >= 0x1100 && (r <= 0x115F || (r >= 0x2E80 && r <= 0x9FFF) ||
		(r >= 0xAC00 && r <= 0xD7A3) || (r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE10 && r <= 0xFE6F) || (r >= 0xFF01 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) || (r >= 0x20000 && r <= 0x3FFFF)) {
		return 2
	}
	return 1
}
