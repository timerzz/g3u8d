package decode

import (
	"os"
	"testing"
)

func TestDecode(t *testing.T) {
	b, err := os.ReadFile("D:\\workspace\\github\\github.com/timerzz/g3u8d\\media\\287570.ts")
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Create("D:\\workspace\\github\\github.com/timerzz/g3u8d\\media\\out.ts")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	key, err := os.ReadFile("D:\\workspace\\github\\github.com/timerzz/g3u8d\\media\\00e7dbc260b688f8.ts")
	if err != nil {
		t.Fatal(err)
	}
	out, err := AESDecrypt(b, key, []byte("0x62dafe649b36307e2a4caf5d30d18490"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Write(out); err != nil {
		t.Fatal(err)
	}
}

func TestEncode(t *testing.T) {
	before := []byte("testtesttestt")
	key := []byte("key1key2key3key4")
	t.Log(before)
	encode, err := AesEncrypt(before, key)
	if err != nil {
		t.Fatal(encode)
	}
	t.Log(encode)
	decode, err := AESDecrypt(encode, key, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(decode)
}
