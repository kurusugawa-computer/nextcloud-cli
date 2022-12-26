package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/thamaji/cachedir"
)

var secretkey = []byte("12345678")

func Clean(appname string) error {
	dir, err := cachedir.Dir()
	if err != nil {
		return err
	}

	return os.RemoveAll(filepath.Join(dir, appname))
}

func Init(appname string) error {
	dir, err := cachedir.Dir()
	if err != nil {
		return err
	}

	dir = filepath.Join(dir, appname)
	path := filepath.Join(dir, "secret.key")

	encoded, err := ioutil.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}

		secretkey = []byte(strconv.Itoa(time.Now().Nanosecond()))

		u, err := user.Current()
		if err != nil {
			return err
		}

		encoded, err := Encode(secretkey, []byte(appname+u.Username))
		if err != nil {
			return err
		}

		return ioutil.WriteFile(path, encoded, 0644)
	}

	u, err := user.Current()
	if err != nil {
		return err
	}

	secretkey, err = Decode(encoded, []byte(appname+u.Username))
	if err != nil {
		return err
	}

	return nil
}

type Password []byte

func (p *Password) String() string {
	return string(*p)
}

func (p *Password) MarshalJSON() ([]byte, error) {
	encoded, err := Encode(*p, secretkey)
	if err != nil {
		return nil, err
	}

	return json.Marshal(string(encoded))
}

func (p *Password) UnmarshalJSON(data []byte) error {
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return err
	}

	decoded, err := Decode([]byte(encoded), secretkey)
	if err != nil {
		return err
	}

	*p = decoded

	return nil
}

type Credential struct {
	URL      string   `json:"url"`
	Username string   `json:"username"`
	Password Password `json:"password"`
}

func Save(appname string, credential *Credential) error {
	if err := Init(appname); err != nil {
		return err
	}

	dir, err := cachedir.Dir()
	if err != nil {
		return err
	}

	dir = filepath.Join(dir, appname)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(filepath.Join(dir, "credential.json"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	err = json.NewEncoder(f).Encode(credential)
	if err1 := f.Close(); err == nil {
		err = err1
	}

	return err
}

func Load(appname string) (*Credential, error) {
	if err := Init(appname); err != nil {
		return nil, err
	}

	dir, err := cachedir.Dir()
	if err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filepath.Join(dir, appname, "credential.json"), os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	credential := Credential{}

	err = json.NewDecoder(f).Decode(&credential)
	if err1 := f.Close(); err == nil {
		err = err1
	}

	return &credential, err
}

func Encode(value []byte, key []byte) ([]byte, error) {
	sum := hmac.New(md5.New, key).Sum(nil)
	k := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(k, sum)

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}

	cipherText := make([]byte, aes.BlockSize+len(value))
	iv := cipherText[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(cipherText[aes.BlockSize:], value)

	buf := make([]byte, base64.StdEncoding.EncodedLen(len(cipherText)))
	base64.StdEncoding.Encode(buf, cipherText)
	return buf, nil
}

func Decode(value []byte, key []byte) ([]byte, error) {
	sum := hmac.New(md5.New, key).Sum(nil)
	k := make([]byte, hex.EncodedLen(len(sum)))
	hex.Encode(k, sum)

	block, err := aes.NewCipher(k)
	if err != nil {
		return nil, err
	}

	cipherBytes := make([]byte, base64.StdEncoding.DecodedLen(len(value)))
	n, err := base64.StdEncoding.Decode(cipherBytes, value)
	if err != nil {
		return nil, err
	}
	cipherBytes = cipherBytes[:n]

	v := make([]byte, len(cipherBytes[aes.BlockSize:]))
	stream := cipher.NewCTR(block, cipherBytes[:aes.BlockSize])
	stream.XORKeyStream(v, cipherBytes[aes.BlockSize:])
	return v, nil
}
