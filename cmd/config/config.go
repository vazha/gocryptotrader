package main

import (
	"flag"
	"io/ioutil"
	"log"

	"github.com/vazha/gocryptotrader/common/file"
	"github.com/vazha/gocryptotrader/config"
)

// EncryptOrDecrypt returns a string from a boolean
func EncryptOrDecrypt(encrypt bool) string {
	if encrypt {
		return "encrypted"
	}
	return "decrypted"
}

func main() {
	var inFile, outFile, key string
	var encrypt bool
	var err error

	configFile, err := config.GetFilePath("")
	if err != nil {
		log.Fatal(err)
	}

	flag.StringVar(&inFile, "infile", configFile, "The config input file to process.")
	flag.StringVar(&outFile, "outfile", configFile+".out", "The config output file.")
	flag.BoolVar(&encrypt, "encrypt", true, "Whether to encrypt or decrypt.")
	flag.StringVar(&key, "key", "", "The key to use for AES encryption.")
	flag.Parse()

	log.Println("GoCryptoTrader: config-helper tool.")

	if key == "" {
		result, errf := config.PromptForConfigKey(false)
		if errf != nil {
			log.Fatal("Unable to obtain encryption/decryption key.")
		}
		key = string(result)
	}

	fileData, err := ioutil.ReadFile(inFile)
	if err != nil {
		log.Fatalf("Unable to read input file %s. Error: %s.", inFile, err)
	}

	if config.ConfirmECS(fileData) && encrypt {
		log.Println("File is already encrypted. Decrypting..")
		encrypt = false
	}

	if !config.ConfirmECS(fileData) && !encrypt {
		var result interface{}
		errf := config.ConfirmConfigJSON(fileData, result)
		if errf != nil {
			log.Fatal("File isn't in JSON format")
		}
		log.Println("File is already decrypted. Encrypting..")
		encrypt = true
	}

	var data []byte
	if encrypt {
		data, err = config.EncryptConfigFile(fileData, []byte(key))
		if err != nil {
			log.Fatalf("Unable to encrypt config data. Error: %s.", err)
		}
	} else {
		data, err = config.DecryptConfigFile(fileData, []byte(key))
		if err != nil {
			log.Fatalf("Unable to decrypt config data. Error: %s.", err)
		}
	}

	err = file.Write(outFile, data)
	if err != nil {
		log.Fatalf("Unable to write output file %s. Error: %s", outFile, err)
	}
	log.Printf(
		"Successfully %s input file %s and wrote output to %s.\n",
		EncryptOrDecrypt(encrypt), inFile, outFile,
	)
}
