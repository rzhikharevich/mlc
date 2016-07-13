package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
	"golang.org/x/crypto/scrypt"
)

var TLSEnabled = false

var secureCookie *securecookie.SecureCookie

func init() {
	secureCookie = securecookie.New(securecookie.GenerateRandomKey(64), nil)
}

func SetSecureCookie(w http.ResponseWriter, name string, value string) (err error) {
	enc, err := secureCookie.Encode(name, value)
	if err != nil {
		return
	}

	cookie := http.Cookie{
		Name:     name,
		Value:    enc,
		Path:     "/",
		Secure:   TLSEnabled,
		HttpOnly: true,
	}

	//fmt.Println(cookie)

	http.SetCookie(w, &cookie)

	return
}

func GetSecureCookie(r *http.Request, name string) (value string, err error) {
	cookie, err := r.Cookie(name)
	if err != nil {
		return
	}

	//fmt.Println(cookie)

	if err = secureCookie.Decode(name, cookie.Value, &value); err != nil {
		return
	}

	return
}

func AuthPlace(w http.ResponseWriter, place string) (err error) {
	val := place + ";" + time.Now().Format(http.TimeFormat) + ";" + GenCSRFToken()

	err = SetSecureCookie(w, "mlc-place-auth", val)

	return
}

func ParsePlaceAuth(r *http.Request) (place, csrf string, err error) {
	val, err := GetSecureCookie(r, "mlc-place-auth")
	if err != nil {
		return
	}

	data := strings.Split(val, ";")
	if len(data) != 3 {
		err = errors.New("invalid auth token")
		return
	}

	place = data[0]

	t, err := time.Parse(http.TimeFormat, data[1])
	if err != nil {
		return
	}

	// A token lives only 24 hours.
	if time.Now().After(t.AddDate(0, 0, 1)) {
		err = errors.New("auth token expired")
		return
	}

	csrf = data[2]

	return
}

func DeauthPlace(w http.ResponseWriter) {
	SetSecureCookie(w, "mlc-place-auth", "")
}

func CryptPassword(password, salt string) (hash []byte, err error) {
	hash, err = scrypt.Key([]byte(password), []byte(salt), 16384, 8, 1, 16)

	return
}

func CryptNewPassword(password string) (hash []byte, salt string, err error) {
	saltb := make([]byte, 16)
	_, err = rand.Read(saltb)
	if err != nil {
		return
	}

	salt = base64.StdEncoding.EncodeToString(saltb)

	hash, err = CryptPassword(password, salt)

	return
}

func SetPlacePassword(place, password string) (err error) {
	hash, salt, err := CryptNewPassword(password)
	if err != nil {
		return
	}

	err = UpdatePlaceHashAndSalt(place, hash, salt)

	return
}

func VerifyPlacePassword(place, password string) bool {
	hash1, salt, err := GetPlaceHashAndSalt(place)
	if err != nil {
		return false
	}

	hash2, err := CryptPassword(password, salt)
	if err != nil {
		return false
	}

	return bytes.Equal(hash1, hash2)
}

func GenCSRFToken() string {
	t := securecookie.GenerateRandomKey(32)
	return base64.StdEncoding.EncodeToString(t)
}
