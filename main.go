package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	//go:embed AESDecrypt.js
	AESDecryptJS string

	//go:embed secret.html
	secretHtml string
)

func main() {
	// Copy Embed files
	wd, _ := os.Getwd()
	PanicWhenError(CopyEmbedFile(AESDecryptJS, filepath.Join(wd, "static/js/AESDecrypt.js")))
	PanicWhenError(CopyEmbedFile(secretHtml, filepath.Join(wd, "layouts/shortcodes/secret.html")))

	// Call Hugo
	if output, err := exec.Command("hugo").Output(); err != nil {
		log.Fatalf("Hugo failed: %v, output: %v", err, string(output))
	} else {
		log.Println(string(output))
	}

	// Encrypt HTML files
	PanicWhenError(EncryptHTMLFiles(filepath.Join(wd, "public")))
}

func PanicWhenError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func CopyEmbedFile(content string, toFilePath string) error {
	if IsFileExist(toFilePath) {
		return nil
	}

	// Create dir if not exist
	dir := filepath.Dir(toFilePath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return err
	}

	// Create the target file and write the contents
	f, err := os.Create(toFilePath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(content)
	return err
}

func IsFileExist(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return true
}

func EncryptHTMLFiles(dir string) error {
	err := filepath.WalkDir(dir, func(path string, info os.DirEntry, err error) error {
		if strings.ToLower(filepath.Ext(path)) == ".html" && !info.IsDir() {
			err = EncryptHTMLFile(path)
		}
		return err
	})
	return err
}

func EncryptHTMLFile(path string) error {
	// Read HTML File content
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Using goquery to parse HTML
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return err
	}

	// Try to find `secret` element
	secretElements := doc.Find("div#secret")
	if secretElements.Length() == 0 {
		return nil
	}

	// Get encrypted password, and remove from HTML
	password, _ := secretElements.Attr("password")
	secretElements.RemoveAttr("password")

	// Encrypt original content
	innerText := secretElements.Text()
	encryptedContent, err := AESEncrypt(innerText, password)
	if err != nil {
		return fmt.Errorf("AESEncrypt failed: %w", err)
	}
	secretElements.SetText(encryptedContent)

	// Add scripts
	doc.Find("body").AppendHtml(`<script src="https://cdn.jsdelivr.net/npm/marked/marked.min.js"></script>`)
	doc.Find("body").AppendHtml(`<script src="https://cdnjs.cloudflare.com/ajax/libs/crypto-js/3.1.9-1/crypto-js.js"></script>`)
	doc.Find("body").AppendHtml(`<script src="/js/AESDecrypt.js"></script>`)

	// Overwrite original HTML file with modified content
	newHtml, err := doc.Html()
	if err != nil {
		return fmt.Errorf("doc.Html failed: %w", err)
	}

	if err = os.WriteFile(path, []byte(newHtml), 0644); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	return nil
}

func AESEncrypt(plain, password string) (string, error) {
	hash := sha256.New()
	hash.Write([]byte(password))
	encryptedPassword := hash.Sum(nil)

	block, err := aes.NewCipher(encryptedPassword)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	hash1 := sha256.Sum256(encryptedPassword)
	nonce := hash1[:aesgcm.NonceSize()]
	ciphertext := aesgcm.Seal(nil, nonce, []byte(plain), nil)
	return hex.EncodeToString(nonce) + "|" + hex.EncodeToString(ciphertext), nil
}
