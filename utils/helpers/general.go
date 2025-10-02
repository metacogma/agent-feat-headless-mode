package helpers

import (
	// Go Internal Packages

	"archive/zip"
	"bufio"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"agent/errors"
	"agent/logger"

	"github.com/gorilla/schema"
)

// Pass is empty place holder for no-op
func Pass() {
	// do nothing
}

// MD5 returns the MD5 hash of given string
func MD5(text string) string {
	hasher := md5.New()
	if _, err := io.WriteString(hasher, text); err != nil {
		panic(err)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// EscapeSpecialChars replaces special characters in a string with "\\" + the character
func EscapeSpecialChars(input string) string {
	re := regexp.MustCompile(`[^\w]`)
	return re.ReplaceAllString(input, "\\$0")
}

// ReplaceWhitespaceWithPipe replaces whitespace with a pipe character
func ReplaceWhitespaceWithPipe(text string) string {
	re := regexp.MustCompile(`\\ `)
	return re.ReplaceAllString(text, "|")
}

// GetSchemaDecoder returns a new instance of schema.Decoder
func GetSchemaDecoder() *schema.Decoder {
	d := schema.NewDecoder()
	d.IgnoreUnknownKeys(true)
	return d
}

// PrintStruct prints a givens struct in pretty format with indent
func PrintStruct(v any) {
	res, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(res))
}

// Map applies a function to each item in a slice and returns a new slice
func Map[A any, B any](arr []A, f func(A) B) []B {
	result := make([]B, len(arr))
	for i, v := range arr {
		result[i] = f(v)
	}
	return result
}

func ExtractZipFile(zipFilePath string, destDir string) error {
	file, err := os.Open(zipFilePath)
	if err != nil {
		fmt.Println("Failed to open zip file:", err)
		return err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Failed to get file info:", err)
		return err
	}

	zipReader, err := zip.NewReader(file, fileInfo.Size())
	if err != nil {
		fmt.Println("Failed to open zip file:", err)
		return err
	}

	if err := os.MkdirAll(destDir, os.ModePerm); err != nil {
		fmt.Println("Failed to create output directory:", err)
		return err
	}
	for _, file := range zipReader.File {
		filePath := filepath.Join(destDir, file.Name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(filePath, os.ModePerm); err != nil {
				fmt.Println("Failed to create directory:", err)
				return err
			}
		} else {
			if err := extractFile(file, filePath); err != nil {
				fmt.Println("Failed to extract file:", err)
				return err
			}
		}
	}

	return nil
}

func extractFile(file *zip.File, filePath string) error {
	srcFile, err := file.Open()
	if err != nil {
		fmt.Println("file opening error \n", err)
		return err
	}
	defer srcFile.Close()

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		fmt.Println("directory creation error \n", err)
		return err
	}

	destFile, err := os.Create(filePath)
	if err != nil {
		fmt.Println("file creation error \n", err)
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		fmt.Println("file copy error \n", err)
		return err
	}
	return nil
}

func DecodeBase64ToImageAndSave(base64Str, basedir, fileName string) error {

	if idx := strings.Index(base64Str, ","); idx != -1 {
		base64Str = base64Str[idx+1:]
	}

	imgBytes, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return errors.E(errors.Internal, fmt.Errorf("failed to decode base64 string: %w", err))
	}

	if _, err := os.Stat(basedir); os.IsNotExist(err) {
		err := os.MkdirAll(basedir, os.ModePerm)
		if err != nil {
			return errors.E(errors.Internal, fmt.Errorf("failed to create basedir %w", err))
		}
	}
	filePath := filepath.Join(basedir, fileName)

	if err := os.WriteFile(filePath, imgBytes, 0644); err != nil {

		return errors.E(errors.Internal, fmt.Errorf("failed to write img bytes to filepath %w", err))
	}

	if _, err := os.Stat(filePath); err != nil {
		return errors.E(errors.Internal, fmt.Errorf("file '%s' does not exist after writing: %w", filePath, err))
	}

	return nil

}

func IsFileStable(filePath string, maxRetries int, retryInterval time.Duration, fileType ...string) (bool, error) {
	var lastSize int64 = -1
	for i := 0; i < maxRetries; i++ {
		info, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("File does not exist yet. Retrying...")
				time.Sleep(retryInterval)
				continue
			}
			return false, err
		}

		currentSize := info.Size()
		if currentSize == lastSize {
			return true, nil
		}
		lastSize = currentSize
		fmt.Println("File size still changing. Retrying...", fileType)
		time.Sleep(retryInterval)
	}
	return false, fmt.Errorf("file is not stable after %d retries", maxRetries)
}

func StdOutput(stdoutPipe io.ReadCloser) {
	if stdoutPipe == nil {
		fmt.Printf("stdoutPipe is nil\n")
		return
	}
	func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Info("stdout", line)
		}
		if err := scanner.Err(); err != nil {
			logger.Error("error reading stdout", err)
		}
	}()
}

func StdError(stderrPipe io.ReadCloser) {
	if stderrPipe == nil {
		logger.Info("stderrPipe is nil", stderrPipe)
		return
	}
	func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Info("stderr", line)
		}
		if err := scanner.Err(); err != nil {
			logger.Error("error reading stderr", err)
		}
	}()
}
