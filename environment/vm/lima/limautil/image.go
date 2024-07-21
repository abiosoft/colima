package limautil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/sirupsen/logrus"
)

func init() {
	if err := loadImages(); err != nil {
		logrus.Fatal(err)
	}
}

func Image(arch environment.Arch, runtime string) (f limaconfig.File, err error) {
	if imgFile, ok := images[runtime]; ok {
		if img, ok := imgFile[arch.GoArch()]; ok {
			return img, nil
		}
	}

	return f, fmt.Errorf("cannot find %s image for %s runtime", arch, runtime)
}

var images = map[string]imageFiles{}

type imageFiles map[string]limaconfig.File

func loadImages() error {
	filename := "images/images.txt"
	b, err := embedded.Read(filename)
	if err != nil {
		logrus.Fatalf("error reading embedded file: %s", filename)
	}
	return loadImagesFromBytes(b)
}

func loadImagesFromBytes(b []byte) error {
	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		line := scanner.Bytes()
		var arch environment.Arch
		var runtime, url, sha string
		_, err := fmt.Fscan(bytes.NewReader(line), &arch, &runtime, &url, &sha)
		if err != nil && err != io.EOF {
			return err
		}

		// sanitise the value
		arch = arch.Value()

		file := limaconfig.File{Location: url, Arch: arch}
		if sha != "" {
			file.Digest = "sha512:" + sha
		}

		var files = imageFiles{}
		if m, ok := images[runtime]; ok {
			files = m
		}
		files[arch.GoArch()] = file

		images[runtime] = files
	}

	return nil
}
