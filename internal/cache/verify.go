package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

func ExistsAndValid(path, digest string, size int64) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}
	if size >= 0 && info.Size() != size {
		return false
	}
	return VerifyDigest(path, digest) == nil
}

func VerifyDigest(path, digest string) error {
	algo, expect, ok := splitDigest(digest)
	if !ok {
		return invalidDigestError(digest)
	}
	if algo != "sha256" {
		return fmt.Errorf("unsupported digest algo: %s", algo)
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}

	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expect {
		return fmt.Errorf("digest mismatch: %s != %s", actual, expect)
	}
	return nil
}
