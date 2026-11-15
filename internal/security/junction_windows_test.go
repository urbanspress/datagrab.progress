//go:build windows

package security

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf16"
)

const (
	fsctlSetReparsePoint    = 0x000900A4
	ioReparseTagMountPoint  = 0xA0000003
	fileFlagBackupSemantics = 0x02000000
	fileFlagOpenReparse     = 0x00200000
)

func createTestReparseEscape(tRoot, linkName, targetDir, targetFile string) (string, error) {
	linkPath := filepath.Join(tRoot, linkName)
	if err := os.Symlink(targetFile, linkPath); err == nil {
		return linkName, nil
	}
	// Fallback on Windows without Developer Mode: create an NTFS directory junction without admin rights.
	dirLink := filepath.Join(tRoot, linkName+"-dir")
	if err := os.MkdirAll(dirLink, 0o755); err != nil {
		return "", err
	}
	if err := createNTFSJunction(dirLink, targetDir); err != nil {
		return "", err
	}
	return linkName + "-dir/" + filepath.Base(targetFile), nil
}

func createNTFSJunction(junctionDir, targetDir string) error {
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	subName := `\??\` + absTarget
	subUTF16 := utf16.Encode([]rune(subName))
	printUTF16 := utf16.Encode([]rune(absTarget))
	subBytes := make([]byte, len(subUTF16)*2)
	for i, c := range subUTF16 {
		binary.LittleEndian.PutUint16(subBytes[i*2:], c)
	}
	printBytes := make([]byte, len(printUTF16)*2)
	for i, c := range printUTF16 {
		binary.LittleEndian.PutUint16(printBytes[i*2:], c)
	}

	pathBufLen := len(subBytes) + 2 + len(printBytes) + 2
	reparseDataLen := 8 + pathBufLen
	buf := make([]byte, 8+reparseDataLen)
	binary.LittleEndian.PutUint32(buf[0:4], ioReparseTagMountPoint)
	binary.LittleEndian.PutUint16(buf[4:6], uint16(reparseDataLen))
	binary.LittleEndian.PutUint16(buf[6:8], 0)
	binary.LittleEndian.PutUint16(buf[8:10], 0)
	binary.LittleEndian.PutUint16(buf[10:12], uint16(len(subBytes)))
	binary.LittleEndian.PutUint16(buf[12:14], uint16(len(subBytes)+2))
	binary.LittleEndian.PutUint16(buf[14:16], uint16(len(printBytes)))
	copy(buf[16:], subBytes)
	copy(buf[16+len(subBytes)+2:], printBytes)

	p16, err := syscall.UTF16PtrFromString(junctionDir)
	if err != nil {
		return err
	}
	h, err := syscall.CreateFile(
		p16,
		syscall.GENERIC_WRITE,
		0,
		nil,
		syscall.OPEN_EXISTING,
		fileFlagBackupSemantics|fileFlagOpenReparse,
		0,
	)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)

	var returned uint32
	return syscall.DeviceIoControl(h, fsctlSetReparsePoint, &buf[0], uint32(len(buf)), nil, 0, &returned, nil)
}
