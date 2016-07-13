package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/howeyc/gopass"
)

func HTTPError(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}

func HTTPRedirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusFound)
}

func main() {
	var err error

	/*
	 * Create the error logger.
	 */

	e := log.New(os.Stderr, "Error: ", 0)

	/*
	 * Parse arguments.
	 */

	user := flag.String(
		"user", "root",
		"MySQL user name",
	)

	host := flag.String(
		"host", "127.0.0.1:3306",
		"MySQL host IP address and port",
	)

	dbName := flag.String(
		"database", "main",
		"MySQL database name",
	)

	httpPort := flag.String(
		"http-port", "80",
		"HTTP port to listen",
	)

	resPath := flag.String(
		"res-path", ".",
		"web resource path",
	)

	newPlaceName := flag.String(
		"add-place", "",
		"add a new place",
	)

	tlsCert := flag.String(
		"tls-cert", "",
		"TLS certificate path, use with -tls-keys",
	)

	tlsKeyFile := flag.String(
		"tls-keys", "",
		"TLS key file path, use with -tls-cert",
	)

	password := flag.String(
		"password", "",
		"MySQL password",
	)

	flag.Parse()

	if (len(*tlsCert) > 0) != (len(*tlsKeyFile) > 0) {
		e.Println("-tls-cert and -tls-keys must be used together")
		flag.Usage()
		return
	}

	/*
	 * Fetch the database password.
	 */

	passwordString := *password

	if len(passwordString) == 0 {
		fmt.Print("Enter MySQL password: ")
		passwordBytes, err := gopass.GetPasswd()
		if err != nil {
			e.Fatalln(err.Error())
		}

		passwordString = string(passwordBytes)
	}

	/*
	 * Connect to the database.
	 */

	err = InitDB(*user, passwordString, *host, *dbName)
	if err != nil {
		e.Fatalln(err.Error())
	}

	defer QuitDB()

	/*
	 * Create a new place if requested.
	 */

	if len(*newPlaceName) > 0 {
		fmt.Print("Enter new place password: ")
		p, err := gopass.GetPasswd()
		if err != nil {
			e.Fatalln(err.Error())
		}

		err = AddPlace(*newPlaceName, string(p))
		if err != nil {
			e.Fatalln(err.Error())
		}

		fmt.Println("Sucessfully added a place with name '" + *newPlaceName + "'!")
		return
	}

	/*
	 * Load and prepare resources.
	 */

	/*
	 * Static resources.
	 */

	err = HandleAllResources(*resPath)
	if err != nil {
		e.Fatalln(err.Error())
	}

	/*
	 * Auth page.
	 */

	placeAuthHTML := LoadResourceOrDie(*resPath+"/place/auth.html", e)

	/*
	 * Main menu page.
	 */

	mainHTML := LoadResourceOrDie(*resPath+"/place/main.html", e)
	mainSpl := TextPlaceholderSplit(mainHTML.data)

	/*
	 * Cashier operational panel.
	 */

	placeOpHTML := LoadResourceOrDie(*resPath+"/place/op.html", e)
	placeOpSpl := TextPlaceholderSplit(placeOpHTML.data)

	/*
	 * Operation success page.
	 */

	opSuccessHTML := LoadResourceOrDie(*resPath+"/place/op_success.html", e)
	opSuccessSpl := TextPlaceholderSplit(opSuccessHTML.data)

	/*
	 * Operation failure page.
	 */

	errorHTML := LoadResourceOrDie(*resPath+"/place/error.html", e)

	/*
	 * Card report request page.
	 */

	cardInfoHTML := LoadResourceOrDie(*resPath+"/place/card_info.html", e)
	cardInfoSpl := TextPlaceholderSplit(cardInfoHTML.data)

	/*
	 * Card report page.
	 */

	cardInfoShowHTML := LoadResourceOrDie(*resPath+"/place/card_info_show.html", e)
	cardInfoShowSpl := TextPlaceholderSplit(cardInfoShowHTML.data)

	/*
	 * Admin: user creation page.
	 */

	newUserHTML := LoadResourceOrDie(*resPath+"/admin/new_user.html", e)
	newUserSpl := TextPlaceholderSplit(newUserHTML.data)

	/*
	 * Admin: user creation success.
	 */

	adminSuccessHTML := LoadResourceOrDie(*resPath+"/admin/success.html", e)
	adminSuccessSpl := TextPlaceholderSplit(adminSuccessHTML.data)

	/*
	 * Monthly report page.
	 */

	reportHTML := LoadResourceOrDie(*resPath+"/admin/report.html", e)
	reportSpl := TextPlaceholderSplit(reportHTML.data)

	/*
	 * Set request handlers.
	 */

	/*
	 * Authentification.
	 */

	http.HandleFunc("/place/auth", func(w http.ResponseWriter, r *http.Request) {
		dst := "/place/main"

		if place, _, err := ParsePlaceAuth(r); err == nil {
			if place == "admin" {
				dst = "/admin/new_user"
			}

			HTTPRedirect(w, r, dst)

			return
		}

		if !VerifyPlacePassword(r.FormValue("place"), r.FormValue("password")) {
			placeAuthHTML.ServeHTTP(w, r)
			return
		}

		AuthPlace(w, r.FormValue("place"))

		if r.FormValue("place") == "admin" {
			dst = "/admin/new_user"
		}

		HTTPRedirect(w, r, dst)
	})

	/*
	 * Main menu.
	 */

	http.HandleFunc("/place/main", func(w http.ResponseWriter, r *http.Request) {
		_, csrf, err := ParsePlaceAuth(r)
		if err != nil {
			HTTPRedirect(w, r, "/place/auth")
			return
		}

		res, err := mainHTML.CopyWithInserted(mainSpl, csrf)
		if err != nil {
			e.Println("mainHTML: " + err.Error())
			HTTPError(w, http.StatusInternalServerError)
			return
		}

		res.ServeHTTP(w, r)
	})

	/*
	 * Cashier operational panel.
	 */

	http.HandleFunc("/place/op", func(w http.ResponseWriter, r *http.Request) {
		place, csrf, err := ParsePlaceAuth(r)
		if err != nil {
			HTTPRedirect(w, r, "/place/auth")
			return
		}

		if r.FormValue("CSRFToken") != csrf {
			res, err := placeOpHTML.CopyWithInserted(placeOpSpl, csrf)
			if err != nil {
				e.Println("placeOpHTML: " + err.Error())
				HTTPError(w, http.StatusInternalServerError)
			} else {
				res.ServeHTTP(w, r)
			}

			return
		}

		cardStr := r.FormValue("card")
		amountStr := r.FormValue("amount")
		cashStr := r.FormValue("plus")

		card, err1 := ParseUserID(cardStr)
		amount, err2 := ParseCashCount(amountStr)
		cash, err3 := ParseCashCount(cashStr)

		if err1 != nil || err2 != nil || err3 != nil {
			HTTPRedirect(w, r, "/place/error")
			return
		}

		discount, err := AddOperation(place, UserID(card), amount, cash)

		if err != nil {
			HTTPRedirect(w, r, "/place/error")
			return
		}

		name, err := GetCardHolderName(card)

		total := amount - discount

		res, err := opSuccessHTML.CopyWithInserted(
			opSuccessSpl,
			name,
			cardStr,
			amountStr,
			FormatCashCount(discount*100/amount), FormatCashCount(discount),
			amountStr,
			FormatCashCount(total),
			FormatCashCount(cash-total),
		)

		if err != nil {
			e.Println("successHTML: " + err.Error())
			HTTPError(w, http.StatusInternalServerError)
		} else {
			res.ServeHTTP(w, r)
		}
	})

	/*
	 * Operation failure.
	 */

	http.Handle("/place/error", errorHTML)

	/*
	 * Card report generation.
	 */

	http.HandleFunc("/place/card_info", func(w http.ResponseWriter, r *http.Request) {
		_, csrf, err := ParsePlaceAuth(r)
		if err != nil {
			HTTPRedirect(w, r, "/place/auth")
			return
		}

		if r.FormValue("CSRFToken") != csrf {
			res, err := cardInfoHTML.CopyWithInserted(cardInfoSpl, csrf)
			if err != nil {
				HTTPError(w, http.StatusInternalServerError)
				return
			}

			res.ServeHTTP(w, r)

			return
		}

		cardStr := r.FormValue("card")

		card, err := ParseUserID(cardStr)
		if err != nil {
			HTTPRedirect(w, r, "/place/error")
			return
		}

		name, phone, mail, balance, count, gender, err := GetFullCardInfo(card)
		if err != nil {
			HTTPRedirect(w, r, "/place/error")
			return
		}

		res, err := cardInfoShowHTML.CopyWithInserted(
			cardInfoShowSpl,
			name,
			cardStr,
			phone,
			mail,
			FormatCashCount(balance),
			FormatCashCount(GetMLCDiscountPercent(balance)),
			strconv.FormatUint(uint64(count), 10),
			FormatGender(gender),
		)
		if err != nil {
			HTTPError(w, http.StatusInternalServerError)
			return
		}

		res.ServeHTTP(w, r)
	})

	/*
	 * Admin: user creation.
	 */

	http.HandleFunc("/admin/new_user", func(w http.ResponseWriter, r *http.Request) {
		place, csrf, err := ParsePlaceAuth(r)
		if err != nil {
			HTTPRedirect(w, r, "/place/auth")
			return
		}

		if place != "admin" {
			HTTPRedirect(w, r, "/place/main")
			return
		}

		if r.FormValue("CSRFToken") != csrf {
			res, err := newUserHTML.CopyWithInserted(newUserSpl, csrf)
			if err != nil {
				e.Println("newUserHTML: " + err.Error())
				HTTPError(w, http.StatusInternalServerError)
				return
			}

			res.ServeHTTP(w, r)

			return
		}

		balance, err := ParseCashCount(r.FormValue("balance"))
		if err != nil {
			HTTPRedirect(w, r, "/place/error")
			return
		}

		var gender Gender
		switch r.FormValue("gender") {
		case "m":
			gender = GenderMale
		case "f":
			gender = GenderFemale
		default:
			HTTPRedirect(w, r, "/place/error")
			return
		}

		id, err := AddCard(r.FormValue("name"), r.FormValue("phone"), r.FormValue("mail"), balance, gender)
		if err != nil {
			HTTPError(w, http.StatusInternalServerError)
			return
		}

		res, err := adminSuccessHTML.CopyWithInserted(
			adminSuccessSpl,
			FormatUserID(id),
		)
		if err != nil {
			HTTPError(w, http.StatusInternalServerError)
			return
		}

		res.ServeHTTP(w, r)
	})

	/*
	 * Admin: report generation.
	 */

	http.HandleFunc("/admin/report", func(w http.ResponseWriter, r *http.Request) {
		res, err := reportHTML.CopyWithInserted(reportSpl, "")
		if err != nil {
			HTTPError(w, http.StatusInternalServerError)
			return
		}

		res.ServeHTTP(w, r)
	})

	/*
	 * Logout.
	 */

	http.HandleFunc("/place/logout", func(w http.ResponseWriter, r *http.Request) {
		_, csrf, err := ParsePlaceAuth(r)
		if err == nil && r.FormValue("CSRFToken") == csrf {
			DeauthPlace(w)
			HTTPRedirect(w, r, "/place/auth")
		} else {
			HTTPRedirect(w, r, "/place/main")
		}
	})

	/*
	 * Start serving requests with HTTP or HTTPS.
	 */

	fmt.Println("Starting to listen on port " + *httpPort + ".")

	if len(*tlsCert) > 0 {
		TLSEnabled = true
		err = http.ListenAndServeTLS(":"+*httpPort, *tlsCert, *tlsKeyFile, nil)
	} else {
		fmt.Println("Warning: TLS is not enabled")
		err = http.ListenAndServe(":"+*httpPort, nil)
	}

	e.Fatalln(err.Error())
}
