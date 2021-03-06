package soft

import (
	"errors"
	"crypto/rand"
	"encoding/json"
	"github.com/boltdb/bolt"
	"github.com/pagarme/cryptokit"
)

type Session struct {
	db *bolt.DB
}

func (s *Session) FindKey(id string) (cryptokit.Key, bool, error) {
	var bytes []byte

	err := s.db.View(func (tx *bolt.Tx) error {
		b := tx.Bucket([]byte("keys"))

		bytes = b.Get([]byte(id))

		return nil
	})

	if err != nil {
		return nil, false, err
	}

	if bytes == nil || len(bytes) == 0 {
		return nil, false, nil
	}

	attribs := make(map[string]interface{})

	if err := json.Unmarshal(bytes, &attribs); err != nil {
		return nil, true, err
	}

	return loadKey(s, attribs), true, nil
}

func (s *Session) Encrypt(mech cryptokit.Mechanism, key cryptokit.Key, in []byte) ([]byte, error) {
	if key.Attributes().Capabilities & cryptokit.Encrypt == 0 {
		return nil, errors.New("Key can't be used for encryption")
	}

	return s.encryptCore(mech, key, in)
}

func (s *Session) Decrypt(mech cryptokit.Mechanism, key cryptokit.Key, in []byte) ([]byte, error) {
	if key.Attributes().Capabilities & cryptokit.Decrypt == 0 {
		return nil, errors.New("Key can't be used for decryption")
	}

	return s.decryptCore(mech, key, in)
}

func (s *Session) Translate(mech cryptokit.Mechanism, inKey cryptokit.Key, in []byte, outKey cryptokit.Key) ([]byte, error) {
	data, err := s.Decrypt(mech, inKey, in)

	if err != nil {
		return nil, err
	}

	return s.Encrypt(mech, outKey, data)
}

func (s *Session) Wrap(mech cryptokit.Mechanism, key, kek cryptokit.Key) ([]byte, error) {
	if kek.Attributes().Capabilities & cryptokit.Wrap == 0 {
		return nil, errors.New("Key can't be used for wrapping")
	}

	return s.encryptCore(mech, kek, key.(*Key).data)
}

func (s *Session) Unwrap(mech cryptokit.Mechanism, key []byte, kek cryptokit.Key, attributes cryptokit.KeyAttributes) (cryptokit.Key, error) {
	if kek.Attributes().Capabilities & cryptokit.Unwrap == 0 {
		return nil, errors.New("Key can't be used for unwrapping")
	}

	if err := s.checkConsistency(attributes); err != nil {
		return nil, err
	}

	data, err := s.decryptCore(mech, kek, key)

	if err != nil {
		return nil, err
	}

	return s.createKey(attributes, data)
}

func (s *Session) Generate(mech cryptokit.Mechanism, attributes cryptokit.KeyAttributes) (cryptokit.Key, error) {
	if err := s.checkConsistency(attributes); err != nil {
		return nil, err
	}

	data := make([]byte, attributes.Length)

	switch mech.(type) {
	case *cryptokit.RandomMechanism:
		_, err := rand.Read(data)

		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("Unsupported mechanism")
	}

	return s.createKey(attributes, data)
}

func (s *Session) Derive(mech cryptokit.Mechanism, key cryptokit.Key, attributes cryptokit.KeyAttributes) (cryptokit.Key, error) {
	if key.Attributes().Capabilities & cryptokit.Derive == 0 {
		return nil, errors.New("Key can't be used for derivation")
	}

	if err := s.checkConsistency(attributes); err != nil {
		return nil, err
	}

	data := make([]byte, attributes.Length)

	switch mech.(type) {
	case *cryptokit.RandomMechanism:
		_, err := rand.Read(data)

		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("Unsupported mechanism")
	}

	return s.createKey(attributes, data)
}

func (s *Session) Close() error {
	return nil
}

func (s *Session) checkConsistency(a cryptokit.KeyAttributes) error {
	if a.Length <= 0 {
		return errors.New("Invalid key size")
	}

	return nil
}

func (s *Session) createKey(a cryptokit.KeyAttributes, data []byte) (*Key, error) {
	k := newKey(s, a, data)

	if k.permanent {
		if err := k.save(); err != nil {
			return nil, err
		}
	}

	return k, nil
}

func (s *Session) encryptCore(mech cryptokit.Mechanism, key cryptokit.Key, in []byte) ([]byte, error) {
	switch v := mech.(type) {
	case cryptokit.BlockCipher:
		return processBlockCipher(v, key, in, true)
	}

	return nil, errors.New("Unknown mechanism")
}

func (s *Session) decryptCore(mech cryptokit.Mechanism, key cryptokit.Key, in []byte) ([]byte, error) {
	switch v := mech.(type) {
	case cryptokit.BlockCipher:
		return processBlockCipher(v, key, in, false)
	}

	return nil, errors.New("Unknown mechanism")
}
