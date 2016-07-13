package main

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type UserID uint32

func ParseUserID(s string) (UserID, error) {
	c, err := strconv.ParseUint(s, 0, 32)
	return UserID(c), err
}

func FormatUserID(id UserID) string {
	return strconv.FormatUint(uint64(id), 10)
}

type CashCount uint32

func ParseCashCount(s string) (CashCount, error) {
	c, err := strconv.ParseUint(s, 0, 32)
	return CashCount(c), err
}

func FormatCashCount(c CashCount) string {
	return strconv.FormatUint(uint64(c), 10)
}

type Gender bool

const GenderMale = true
const GenderFemale = false

func GenderRaw(gender Gender) bool {
	return bool(gender)
}

func FormatGender(gender Gender) string {
	switch gender {
	case GenderMale:
		return "мужской"
	case GenderFemale:
		return "женский"
	default:
		return "неизвестен"
	}
}

var db *sql.DB

func InitDB(user string, password string, host string, dbName string) (err error) {
	db, err = sql.Open(
		"mysql",
		user+":"+password+"@tcp("+host+")/"+dbName,
	)

	if err != nil {
		return err
	}

	return db.Ping()
}

func QuitDB() {
	db.Close()
}

func PlaceExists(place string) bool {
	var exists int

	err := db.QueryRow("SELECT COUNT(1) FROM places WHERE name = ?", place).Scan(&exists)
	if err != nil || exists != 1 {
		return false
	}

	return true
}

func UserExists(user UserID) bool {
	var exists int

	err := db.QueryRow("SELECT COUNT(1) FROM cards WHERE id = ?", user).Scan(&exists)
	if err != nil || exists != 1 {
		return false
	}

	return true
}

func InsertPlace(id UserID, place string, hash []byte, salt string) (err error) {
	_, err = db.Exec(
		"INSERT INTO places VALUES(?, ?, ?, ?, 0)",
		id,
		place,
		hash,
		salt,
	)

	return
}

func AddPlace(place, password string) (err error) {
	hash, salt, err := CryptNewPassword(password)
	if err != nil {
		return
	}

	err = InsertPlace(CountPlaces(), place, hash, salt)

	return
}

func GetPlaceHashAndSalt(place string) (hash []byte, salt string, err error) {
	hash = make([]byte, 16)

	err = db.QueryRow(
		"SELECT password_hash, password_salt FROM places WHERE name = ?",
		place,
	).Scan(&hash, &salt)
	if err != nil {
		return
	}

	return
}

func UpdatePlaceHashAndSalt(place string, hash []byte, salt string) (err error) {
	_, err = db.Exec(
		"UPDATE places SET password_hash_lo = ?, password_hash_hi = ?, password_salt = ? WHERE name = ?",
		binary.LittleEndian.Uint64(hash[:8]),
		binary.LittleEndian.Uint64(hash[8:]),
		salt,
		place,
	)

	return
}

func GetPlaceCash(place string) (cash CashCount, err error) {
	err = db.QueryRow(
		"SELECT cash FROM places WHERE name = ?",
		place,
	).Scan(&cash)

	return
}

func AddPlaceCash(place_id UserID, plus CashCount) (err error) {
	_, err = db.Exec(
		"UPDATE places SET cash = cash + ? WHERE id = ?",
		plus, place_id,
	)

	return
}

func ClearPlaceCash(place_id UserID) (err error) {
	_, err = db.Exec(
		"UPDATE places SET cash = 0 WHERE id = ?",
		place_id,
	)

	return
}

func CountRows(table string) (c UserID) {
	err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&c)
	if err != nil {
		panic(err.Error())
	}

	return
}

func CountPlaces() UserID {
	return CountRows("places")
}

func CountOperations() UserID {
	return CountRows("operations")
}

func CountCards() UserID {
	return CountRows("cards")
}

func GetPlaceID(place string) (id UserID, err error) {
	err = db.QueryRow(
		"SELECT id FROM places WHERE name = ?",
		place,
	).Scan(&id)

	return
}

func AddCard(name string, phone string, mail string, balance CashCount, gender Gender) (id UserID, err error) {
	id = CountCards()

	_, err = db.Exec(
		"INSERT INTO cards VALUES(?, ?, ?, ?, ?, ?, ?)",
		id, name, phone, mail, balance, 0, GenderRaw(gender),
	)

	return
}

func GetCardHolderName(id UserID) (s string, err error) {
	err = db.QueryRow(
		"SELECT name FROM cards WHERE id = ?",
		id,
	).Scan(&s)

	return
}

func GetFullCardInfo(id UserID) (name string, phone string, mail string, balance CashCount, count uint32, gender Gender, err error) {
	var rawGender bool

	err = db.QueryRow(
		"SELECT name, phone, mail, balance, count, gender FROM cards WHERE id = ?", id,
	).Scan(&name, &phone, &mail, &balance, &count, &rawGender)

	gender = Gender(rawGender)

	if err != nil {
		fmt.Println(err.Error())
	}

	return
}

func GetCardBalance(id UserID) (b CashCount, err error) {
	err = db.QueryRow(
		"SELECT balance FROM cards WHERE id = ?",
		id,
	).Scan(&b)

	return
}

func AddCardBalance(id UserID, plus CashCount) (err error) {
	_, err = db.Exec(
		"UPDATE cards SET balance = balance + ? WHERE id = ?",
		plus, id,
	)

	return
}

func IncrementCardCount(id UserID) (err error) {
	_, err = db.Exec(
		"UPDATE cards SET count = count + 1 WHERE id = ?",
		id,
	)

	return
}

func InsertOperation(card_id, place_id UserID, amount, discount, cash_plus CashCount) (err error) {
	_, err = db.Exec(
		"INSERT INTO operations VALUES(?, ?, ?, ?, ?, ?, ?)",
		CountOperations(), card_id, place_id,
		amount, discount, cash_plus,
		time.Now(),
	)

	return
}

func AddOperation(place string, card UserID, amount, cash CashCount) (discount CashCount, err error) {
	b, err := GetCardBalance(card)
	if err != nil {
		return
	}

	place_id, err := GetPlaceID(place)
	if err != nil {
		return
	}

	discount = amount / 100 * GetMLCDiscountPercent(b)

	err = InsertOperation(
		card, place_id,
		amount, discount, cash,
	)
	if err != nil {
		return
	}

	AddPlaceCash(place_id, discount)

	AddCardBalance(card, amount)
	IncrementCardCount(card)

	return
}
