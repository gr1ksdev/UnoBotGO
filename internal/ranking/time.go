package ranking

import (
	"time"
	_ "time/tzdata"
)

// RankingLocation is the canonical business timezone for month transitions (America/Sao_Paulo).
var RankingLocation = func() *time.Location {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic("ranking: failed to load timezone America/Sao_Paulo: " + err.Error())
	}
	return loc
}()

var monthNamesPtBR = [...]string{
	1:  "Janeiro",
	2:  "Fevereiro",
	3:  "Março",
	4:  "Abril",
	5:  "Maio",
	6:  "Junho",
	7:  "Julho",
	8:  "Agosto",
	9:  "Setembro",
	10: "Outubro",
	11: "Novembro",
	12: "Dezembro",
}

// MonthName returns the Portuguese name for the calendar month of t in America/Sao_Paulo.
func MonthName(t time.Time) string {
	inLoc := t.In(RankingLocation)
	m := int(inLoc.Month())
	if m >= 1 && m <= 12 {
		return monthNamesPtBR[m]
	}
	return ""
}

// MonthStart returns midnight at the 1st of the month in America/Sao_Paulo.
func MonthStart(t time.Time) time.Time {
	inLoc := t.In(RankingLocation)
	return time.Date(inLoc.Year(), inLoc.Month(), 1, 0, 0, 0, 0, RankingLocation)
}

// MonthDateString returns the YYYY-MM-DD date string of MonthStart for SQL date binding.
func MonthDateString(t time.Time) string {
	return MonthStart(t).Format("2006-01-02")
}
