package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"unicode"
  "github.com/google/go-cmp/cmp"
	"unicode/utf8"
)

type Encoder struct {
	Encoder    map[string]int
	Decoder    map[int]string
	ByteEncode map[rune]rune
	ByteDecode map[rune]rune
	BpeRanks   map[[2]string]int
}

func RuneSliceFromString(s string) []rune {
	runes := []rune{}
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		runes = append(runes, r)
		s = s[size:]
	}
	return runes
}

func GetEncoder(filenameEncoder string, filenameBPE string) (*Encoder, error) {
	file, err := os.Open(filenameEncoder)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValueEncoder, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var fEncoder map[string]int
	err = json.Unmarshal(byteValueEncoder, &fEncoder)
	if err != nil {
		return nil, err
	}

	decoder := make(map[int]string)
	for k, v := range fEncoder {
		decoder[v] = k
	}

	file2, err := os.Open(filenameBPE)
	if err != nil {
		return nil, err
	}
	defer file2.Close()

	byteValueBPE, err := ioutil.ReadAll(file2)
	if err != nil {
		return nil, err
	}

	fBPE := strings.Split(string(byteValueBPE), "\n")[1:]
	bpeMerges := make(map[[2]string]int)
	for i, mergeStr := range fBPE {
		s := strings.Fields(mergeStr)
		if len(s) == 2 {
			bpeMerges[[2]string{s[0], s[1]}] = i
		}
	}

	byteEncoder := make(map[rune]rune)
	byteDecoder := make(map[rune]rune)

	for i, j := 0, 0; i < 256 || j < 256; i++ {
		if unicode.IsPrint(rune(i)) && i != j {
			byteEncoder[rune(j)] = rune(i)
			byteDecoder[rune(i)] = rune(j)
			j++
		} else if i != j {
			byteEncoder[rune(i)] = rune(256 + j)
			byteDecoder[rune(256+j)] = rune(i)
			j++
		} else {
			byteEncoder[rune(i)] = rune(i)
			byteDecoder[rune(i)] = rune(i)
		}
	}

	return &Encoder{
		Encoder:    fEncoder,
		Decoder:    decoder,
		ByteEncode: byteEncoder,
		ByteDecode: byteDecoder,
		BpeRanks:   bpeMerges,
	}, nil
}

func GetPairs(word string) [][2]string {
	runes := RuneSliceFromString(word)
	pairs := [][2]string{}
	for i := 1; i < len(runes); i++ {
		pairs = append(pairs, [2]string{string(runes[i-1]), string(runes[i])})
	}
	return pairs
}

func (e *Encoder) BPE(token string) string {
    runes := RuneSliceFromString(token)
    word := make([]string, len(runes))
    for i, r := range runes {
        word[i] = string(r)
    }

    pairs := GetPairs(token)

    if len(pairs) == 0 {
        return token
    }

    for {
        bigramExists := false
        for _, pair := range pairs {
            if _, pairExists := e.BpeRanks[pair]; pairExists {
                bigramExists = true
                first, second := pair[0], pair[1]

                newWord := []string{}
                i, n := 0, len(word)
                for i < n {
                    if word[i] == first && i+1 < n && word[i+1] == second {
                        newWord = append(newWord, first+second)
                        i += 2
                    } else {
                        newWord = append(newWord, word[i])
                        i++
                    }
                }
                word = newWord
                break
            }
        }

        newPairs := GetPairs(strings.Join(word, ""))
        if !bigramExists || len(newPairs) == 0 || cmp.Equal(newPairs, pairs) {
            break
        } else {
            pairs = newPairs
        }
    }

    return strings.Join(word, " ")
}

func (e *Encoder) Encode(text string) []int {
	pat := `\p{L}+|\p{N}+|[^\s\p{L}\p{N}]+|\s+|\S`
	re := regexp.MustCompile(pat)
	tokens := re.FindAllString(text, -1)

	var bpeTokens []int
	for _, token := range tokens {
		var encoded string
		for _, r := range token {
			encoded += string(e.ByteEncode[r])
		}

		bpe := e.BPE(encoded)
		for _, s := range strings.Split(bpe, " ") {
			bpeTokens = append(bpeTokens, e.Encoder[s])
		}
	}

	return bpeTokens
}

func (e *Encoder) Decode(tokens []int) string {
	text := ""
	for _, token := range tokens {
		text += e.Decoder[token]
	}

	var decoded string
	for _, r := range text {
		decoded += string(e.ByteDecode[r])
	}

	return decoded
}

func main() {
	encoder, err := GetEncoder("./encoder.json", "./vocab.bpe")
	if err != nil {
		panic(err)
	}

	encodedText := encoder.Encode("hello 👋 world 🌍 This is a long string to test whether or not the emoji issue was fixed!")
	fmt.Println("Encoded:", encodedText)

	decodedText := encoder.Decode(encodedText)
	fmt.Println("Decoded:", decodedText)
}
