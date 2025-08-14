//go:build windows
// +build windows

package watcher

import (
	"os"
	"syscall"
)

func isHiddenFile(path string) (isHidden bool, err error) {

	var pU16 *uint16
	if pU16, err = syscall.UTF16PtrFromString(path); err != nil {
		return
	}
	var attributes uint32
	if attributes, err = syscall.GetFileAttributes(pU16); err != nil {
		return
	}
	isHidden = attributes&syscall.FILE_ATTRIBUTE_HIDDEN != 0
	return
}

func isHiddenFileEx(path string) (isHidden bool, err error) {

	if isHidden, err = isHiddenFile(path); os.IsNotExist(err) {
		err = &os.PathError{Op: "isHidden", Path: path, Err: err}
	}
	return
}
