package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io/ioutil"
)

// To encode publicKey use:
// publicKeyBytes, _ = x509.MarshalPKIXPublicKey(&private_key.PublicKey)

func main() {
	p521 := elliptic.P521()
	priv1, _ := ecdsa.GenerateKey(p521, rand.Reader)

	privateKeyBytes, _ := x509.MarshalECPrivateKey(priv1)
	publicKeyBytes, _ := x509.MarshalPKIXPublicKey(&priv1.PublicKey)

	encodedPrivateBytes := hex.EncodeToString(privateKeyBytes)
	encodedPublicBytes := hex.EncodeToString(publicKeyBytes)

	fmt.Println("Encoded Public key: ", encodedPublicBytes)
	fmt.Println("Encoded Private key: ", encodedPrivateBytes)

	file, _ := ioutil.TempFile(".", "encodedKeys")
	defer file.Close()
	file.WriteString(encodedPublicBytes + "\r\n" + encodedPrivateBytes)

}
