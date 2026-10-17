// Package numbers finds Swedish personnummer, samordningsnummer, and payment card numbers by their structure.
//
// Models judge a number by the label in front of it, so a bare 197306253977 scores low. These checks look
// at the number itself: a real date and a passing Luhn check digit. About one in ten random 10-digit numbers
// that start with a valid date also pass, so a bare phone number can match.
package numbers

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// YYMMDD-NNNN, YYMMDDNNNN, YYYYMMDD-NNNN, or YYYYMMDDNNNN; + replaces - for people over 100.
	personnummerPattern = regexp.MustCompile(`\b(?:\d{2})?\d{6}[-+]?\d{4}\b`)
	// 13 to 19 digits, optionally grouped with spaces or dashes.
	cardPattern = regexp.MustCompile(`\b\d(?:[ -]?\d){12,18}\b`)
)

// Kinds returned by Kind, named as in rubric.TypeOptions.
const (
	GovernmentID = "government_id"
	PaymentCard  = "payment_card"
)

// Kind is GovernmentID or PaymentCard when the text has a valid number of that kind, otherwise "".
func Kind(text string) string {
	for _, m := range personnummerPattern.FindAllString(text, -1) {
		if Personnummer(m) {
			return GovernmentID
		}
	}
	for _, m := range cardPattern.FindAllString(text, -1) {
		if CardNumber(m) {
			return PaymentCard
		}
	}
	return ""
}

// Label puts a label on each valid number, such as [personnummer: 197306253977], so a model reads
// what the number is. Kev scores a bare valid personnummer around 0.4 and the labelled one above 0.9.
func Label(text string) string {
	text = personnummerPattern.ReplaceAllStringFunc(text, func(m string) string {
		if !Personnummer(m) {
			return m
		}
		if samordningsnummer(m) {
			return "[samordningsnummer: " + m + "]"
		}
		return "[personnummer: " + m + "]"
	})
	return cardPattern.ReplaceAllStringFunc(text, func(m string) string {
		if !CardNumber(m) {
			return m
		}
		return "[card number: " + m + "]"
	})
}

// Personnummer reports whether s is a personnummer or samordningsnummer with a real date and check digit.
func Personnummer(s string) bool {
	d := digits(s)
	if len(d) != 10 && len(d) != 12 {
		return false
	}
	ten := d[len(d)-10:]
	if !luhn(ten) {
		return false
	}
	day, _ := strconv.Atoi(ten[4:6])
	mmdd := ten[2:4] + pad2(day%60)
	if len(d) == 12 {
		century := d[:2]
		return (century == "18" || century == "19" || century == "20") && validDate(d[:4]+mmdd)
	}
	return validDate("19"+ten[:2]+mmdd) || validDate("20"+ten[:2]+mmdd)
}

// CardNumber reports whether s is 13 to 19 digits starting 2 to 6 with a passing check digit.
func CardNumber(s string) bool {
	d := digits(s)
	return len(d) >= 13 && len(d) <= 19 && d[0] >= '2' && d[0] <= '6' && luhn(d)
}

func samordningsnummer(s string) bool {
	d := digits(s)
	day, _ := strconv.Atoi(d[len(d)-6 : len(d)-4])
	return day > 60
}

func luhn(d string) bool {
	sum := 0
	for i := 0; i < len(d); i++ {
		n := int(d[len(d)-1-i] - '0')
		if i%2 == 1 {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
	}
	return sum%10 == 0
}

func validDate(yyyymmdd string) bool {
	t, err := time.Parse("20060102", yyyymmdd)
	return err == nil && t.Format("20060102") == yyyymmdd
}

func digits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}
