package pmcrypto

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/packet"
	errors2 "golang.org/x/crypto/openpgp/errors"
	"math"
	)

// DecryptMessage decrypt encrypted message use private key (string )
// encryptedText : string armored encrypted
// privateKey : armored private use to decrypt message
// passphrase : match with private key to decrypt message
func (o *OpenPGP) DecryptMessage(encryptedText string, privateKey string, passphrase string) (string, error) {
	privKeyRaw, err := UnArmor(privateKey)
	if err != nil {
		return "", err
	}
	return o.DecryptMessageBinKey(encryptedText, privKeyRaw, passphrase)
}

// DecryptMessageBinKey decrypt encrypted message use private key (bytes )
// encryptedText : string armored encrypted
// privateKey : unarmored private use to decrypt message could be mutiple keys
// passphrase : match with private key to decrypt message
func (o *OpenPGP) DecryptMessageBinKey(encryptedText string, privateKey []byte, passphrase string) (string, error) {
	privKey := bytes.NewReader(privateKey)
	privKeyEntries, err := openpgp.ReadKeyRing(privKey)
	if err != nil {
		return "", err
	}

	encryptedio, err := unArmor(encryptedText)
	if err != nil {
		return "", err
	}

	rawPwd := []byte(passphrase)
	for _, e := range privKeyEntries {
		if e.PrivateKey != nil && e.PrivateKey.Encrypted {
			e.PrivateKey.Decrypt(rawPwd)
		}

		for _, sub := range e.Subkeys {
			if sub.PrivateKey != nil && sub.PrivateKey.Encrypted {
				sub.PrivateKey.Decrypt(rawPwd)
			}
		}
	}

	config := &packet.Config{ Time: o.getTimeGenerator() }

	md, err := openpgp.ReadMessage(encryptedio.Body, privKeyEntries, nil, config)
	if err != nil {
		return "", err
	}

	decrypted := md.UnverifiedBody
	b, err := ioutil.ReadAll(decrypted)
	if err != nil {
		return "", err
	}

	println(4)
	return string(b), nil
}

// DecryptMessageVerifyPrivbinkeys decrypt message and verify the signature
// veriferKey string: armored verifier keys
// privateKey []byte: unarmored private key to decrypt. could be mutiple
func (o *OpenPGP) DecryptMessageVerifyPrivbinkeys(encryptedText string, veriferKey string, privateKeys []byte, passphrase string, verifyTime int64) (*DecryptSignedVerify, error) {

	if len(veriferKey) > 0 {
		verifierRaw, err := UnArmor(veriferKey)
		if err != nil {
			return nil, err
		}
		return o.decryptMessageVerifyAllBin(encryptedText, verifierRaw, privateKeys, passphrase, verifyTime)
	}
	return o.decryptMessageVerifyAllBin(encryptedText, nil, privateKeys, passphrase, verifyTime)
}

// DecryptMessageVerifyBinKeyPrivbinkeys decrypt message and verify the signature
// veriferKey []byte: unarmored verifier keys
// privateKey []byte: unarmored private key to decrypt. could be mutiple
func (o *OpenPGP) DecryptMessageVerifyBinKeyPrivbinkeys(encryptedText string, veriferKey []byte, privateKeys []byte, passphrase string, verifyTime int64) (*DecryptSignedVerify, error) {
	return o.decryptMessageVerifyAllBin(encryptedText, veriferKey, privateKeys, passphrase, verifyTime)
}

// DecryptMessageVerify decrypt message and verify the signature
// veriferKey string: armored verifier keys
// privateKey string: private to decrypt
func (o *OpenPGP) DecryptMessageVerify(encryptedText string, veriferKey string, privateKey string, passphrase string, verifyTime int64) (*DecryptSignedVerify, error) {
	if len(veriferKey) > 0 {
		verifierRaw, err := UnArmor(veriferKey)
		if err != nil {
			return nil, err
		}
		return o.DecryptMessageVerifyBinKey(encryptedText, verifierRaw, privateKey, passphrase, verifyTime)
	}
	return o.DecryptMessageVerifyBinKey(encryptedText, nil, privateKey, passphrase, verifyTime)
}

// DecryptMessageVerifyBinKey decrypt message and verify the signature
// veriferKey []byte: unarmored verifier keys
// privateKey string: private to decrypt
func (o *OpenPGP) DecryptMessageVerifyBinKey(encryptedText string, veriferKey []byte, privateKey string, passphrase string, verifyTime int64) (*DecryptSignedVerify, error) {
	privateKeyRaw, err := UnArmor(privateKey)
	if err != nil {
		return nil, err
	}
	return o.decryptMessageVerifyAllBin(encryptedText, veriferKey, privateKeyRaw, passphrase, verifyTime)
}

// decryptMessageVerifyAllBin
// decrypt_message_verify_single_key(private_key: string, passphras: string, encrypted : string, signature : string) : decrypt_sign_verify;
// decrypt_message_verify(passphras: string, encrypted : string, signature : string) : decrypt_sign_verify;
func (o *OpenPGP) decryptMessageVerifyAllBin(encryptedText string, veriferKey []byte, privateKey []byte, passphrase string, verifyTime int64) (*DecryptSignedVerify, error) {
	privKey := bytes.NewReader(privateKey)
	privKeyEntries, err := openpgp.ReadKeyRing(privKey)
	if err != nil {
		return nil, err
	}

	rawPwd := []byte(passphrase)
	for _, e := range privKeyEntries {

		if e.PrivateKey != nil && e.PrivateKey.Encrypted {
			e.PrivateKey.Decrypt(rawPwd)
		}

		for _, sub := range e.Subkeys {
			if sub.PrivateKey != nil && sub.PrivateKey.Encrypted {
				sub.PrivateKey.Decrypt(rawPwd)
			}
		}
	}

	out := &DecryptSignedVerify{}
	out.Verify = failed

	var verifierEntries openpgp.EntityList
	if len(veriferKey) > 0 {
		verifierReader := bytes.NewReader(veriferKey)
		verifierEntries, err = openpgp.ReadKeyRing(verifierReader)
		if err != nil {
			return nil, err
		}

		for _, e := range verifierEntries {
			privKeyEntries = append(privKeyEntries, e)
		}
	} else {
		out.Verify = noVerifier
	}

	encryptedio, err := unArmor(encryptedText)
	if err != nil {
		return nil, err
	}

	config := &packet.Config{}
	config.Time = func() time.Time {
		return time.Unix(0, 0)
	}

	md, err := openpgp.ReadMessage(encryptedio.Body, privKeyEntries, nil, config)
	if err != nil {
		return nil, err
	}

	decrypted := md.UnverifiedBody
	b, err := ioutil.ReadAll(decrypted)
	if err != nil {
		return nil, err
	}

	processSignatureExpiration(md, verifyTime)

	out.Plaintext = string(b)
	if md.IsSigned {
		if md.SignedBy != nil {
			if verifierEntries != nil {
				matches := verifierEntries.KeysById(md.SignedByKeyId)
				if len(matches) > 0 {
					if md.SignatureError == nil {
						out.Verify = ok
					} else {
						out.Message = md.SignatureError.Error()
						out.Verify = failed
					}
				}
			} else {
				out.Verify = noVerifier
			}
		} else {
			out.Verify = noVerifier
		}
	} else {
		out.Verify = notSigned
	}
	return out, nil
}

// Handle signature time verification manually, so we can add a margin to the creationTime check.
func processSignatureExpiration(md *openpgp.MessageDetails, verifyTime int64) {
	if md.SignatureError == errors2.ErrSignatureExpired {
		if verifyTime > 0 {
			created := md.Signature.CreationTime.Unix()
			expires := int64(math.MaxInt64)
			if md.Signature.KeyLifetimeSecs != nil {
				expires = int64(*md.Signature.KeyLifetimeSecs) + created
			}
			if created - creationTimeOffset <= verifyTime && verifyTime <= expires {
				md.SignatureError = nil
			}
		} else {
			// verifyTime = 0: time check disabled, everything is okay
			md.SignatureError = nil
		}
	}
}

// EncryptMessage encrypt message with public key, if pass private key and passphrase will also sign the message
// publicKey : string armored public key
// plainText : the input
// privateKey : optional required when you want to sign
// passphrase : optional required when you pass the private key and this passphrase must could decrypt the private key
func (o *OpenPGP) EncryptMessage(plainText string, publicKey string, privateKey string, passphrase string, trim bool) (string, error) {
	rawPubKey, err := UnArmor(publicKey)
	if err != nil {
		return "", err
	}
	return o.EncryptMessageBinKey(plainText, rawPubKey, privateKey, passphrase, trim)
}

// EncryptMessageBinKey encrypt message with unarmored public key, if pass private key and passphrase will also sign the message
// publicKey : bytes unarmored public key
// plainText : the input
// privateKey : optional required when you want to sign
// passphrase : optional required when you pass the private key and this passphrase must could decrypt the private key
func (o *OpenPGP) EncryptMessageBinKey(plainText string, publicKey []byte, privateKey string, passphrase string, trim bool) (string, error) {

	if trim {
		plainText = trimNewlines(plainText)
	}
	var outBuf bytes.Buffer
	w, err := armor.Encode(&outBuf, pgpMessageType, armorHeader)
	if err != nil {
		return "", err
	}

	pubKeyReader := bytes.NewReader(publicKey)
	pubKeyEntries, err := openpgp.ReadKeyRing(pubKeyReader)
	if err != nil {
		return "", err
	}

	var signEntity *openpgp.Entity

	if len(passphrase) > 0 && len(privateKey) > 0 {
		signerReader := strings.NewReader(privateKey)
		signerEntries, err := openpgp.ReadArmoredKeyRing(signerReader)
		if err != nil {
			return "", err
		}

		for _, e := range signerEntries {
			// Entity.PrivateKey must be a signing key
			if e.PrivateKey != nil {
				if e.PrivateKey.Encrypted {
					e.PrivateKey.Decrypt([]byte(passphrase))
				}
				if !e.PrivateKey.Encrypted {
					signEntity = e
					break
				}
			}
		}

		if signEntity == nil {
			return "", errors.New("cannot sign message, singer key is not unlocked")
		}
	}

	config := &packet.Config{DefaultCipher: packet.CipherAES256, Time: o.getTimeGenerator() }

	ew, err := openpgp.Encrypt(w, pubKeyEntries, signEntity, nil, config)

	_, _ = ew.Write([]byte(plainText))
	ew.Close()
	w.Close()
	return outBuf.String(), nil
}

//EncryptMessageWithPassword encrypt a plain text to pgp message with a password
//plainText string: clear text
//output string: armored pgp message
func (o *OpenPGP) EncryptMessageWithPassword(plainText string, password string) (string, error) {

	var outBuf bytes.Buffer
	w, err := armor.Encode(&outBuf, pgpMessageType, armorHeader)
	if err != nil {
		return "", err
	}

	config := &packet.Config{ Time: o.getTimeGenerator() }
	plaintext, err := openpgp.SymmetricallyEncrypt(w, []byte(password), nil, config)
	if err != nil {
		return "", err
	}
	message := []byte(plainText)
	_, err = plaintext.Write(message)
	if err != nil {
		return "", err
	}
	err = plaintext.Close()
	if err != nil {
		return "", err
	}
	w.Close()

	return outBuf.String(), nil
}

//DecryptMessageWithPassword decrypt a pgp message with a password
//encrypted string : armored pgp message
//output string : clear text
func (o *OpenPGP) DecryptMessageWithPassword(encrypted string, password string) (string, error) {
	encryptedio, err := unArmor(encrypted)
	if err != nil {
		return "", err
	}

	var prompt = func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		return []byte(password), nil
	}

	config := &packet.Config{ Time: o.getTimeGenerator() }
	md, err := openpgp.ReadMessage(encryptedio.Body, nil, prompt, config)
	if err != nil {
		return "", err
	}

	messageBuf := bytes.NewBuffer(nil)
	_, err = io.Copy(messageBuf, md.UnverifiedBody)
	if err != nil {
		return "", err
	}

	return messageBuf.String(), nil
}
