package limautil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/downloader"
	"github.com/sirupsen/logrus"
)

func init() {
	if err := loadImages(); err != nil {
		logrus.Fatal(err)
	}
}

// ImageCached returns if the image for architecture and runtime
// has been previously downloaded and cached.
func ImageCached(arch environment.Arch, runtime string) (limaconfig.File, bool) {
	img, err := findImage(arch, runtime)
	if err != nil {
		return img, false
	}

	image := diskImageFile(downloader.CacheFilename(img.Location))

	img.Location = image.Location()
	img.Digest = ""

	return img, image.Generated()
}

func findImage(arch environment.Arch, runtime string) (f limaconfig.File, err error) {
	err = fmt.Errorf("cannot find %s image for %s runtime", arch, runtime)

	imgFile, ok := diskImageMap[runtime]
	if !ok {
		return
	}
	img, ok := imgFile[arch.GoArch()]
	if !ok {
		return
	}
	return img, nil
}

// Image returns the details of the disk image to download for the arch and runtime.
func Image(arch environment.Arch, runtime string) (limaconfig.File, error) {
	return findImage(arch, runtime)
}

// DownloadImage downloads the image for arch and runtime.
func DownloadImage(arch environment.Arch, runtime string) (f limaconfig.File, err error) {
	img, err := findImage(arch, runtime)
	if err != nil {
		return img, err
	}

	host := host.New()
	// download image
	qcow2, err := downloadImage(host, img)
	if err != nil {
		return f, err
	}

	diskImage := diskImageFile(qcow2)

	// if qemu-img is missing, ignore raw conversion
	if err := util.AssertQemuImg(); err != nil {
		img.Location = diskImage.String()
		img.Digest = "" // remove digest
		return img, nil
	}

	// convert from qcow2 to raw
	raw, err := qcow2ToRaw(host, diskImage)
	if err != nil {
		return f, err
	}

	img.Location = raw
	img.Digest = "" // remove digest
	return img, nil
}

// map of runtime to disk images.
var diskImageMap = map[string]diskImages{}

// map of architecture to disk image
type diskImages map[string]limaconfig.File

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

		var files = diskImages{}
		if m, ok := diskImageMap[runtime]; ok {
			files = m
		}
		files[arch.GoArch()] = file

		diskImageMap[runtime] = files
	}

	return nil
}

// downloadImage downloads the file and returns the location of the downloaded file.
func downloadImage(host environment.HostActions, file limaconfig.File) (string, error) {
	// download image
	request := downloader.Request{URL: file.Location}
	if file.Digest != "" {
		request.SHA = &downloader.SHA{Size: 512, Digest: file.Digest}
	}
	location, err := downloader.Download(host, request)
	if err != nil {
		return "", fmt.Errorf("error during image download: %w", err)
	}

	return location, nil
}

// qcow2ToRaw uses qemu-img to conver the image from qcow to raw.
// Returns the filename of the raw file and an error (if any).
func qcow2ToRaw(host environment.Host, image diskImageFile) (string, error) {
	if _, err := os.Stat(image.Raw()); err == nil {
		// already exists, return
		return image.Raw(), nil
	}

	err := host.Run("qemu-img", "convert", "-f", "qcow2", "-O", "raw", image.String(), image.Raw())
	if err != nil {
		// remove the incomplete raw file
		_ = host.RunQuiet("rm", "-f", image.Raw())
		return "", err
	}

	return image.Raw(), err
}

type diskImageFile string

func (d diskImageFile) String() string { return strings.TrimSuffix(string(d), ".raw") }
func (d diskImageFile) Raw() string    { return d.String() + ".raw" }
func (d diskImageFile) Generated() bool {
	stat, err := os.Stat(d.Location())
	return err == nil && !stat.IsDir()
}

// Location returns the expected location of the image based on availability of qemu.
func (d diskImageFile) Location() string {
	if err := util.AssertQemuImg(); err == nil {
		return d.Raw()
	}
	return d.String()
}
