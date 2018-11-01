package main

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

func parseDuration(input string) (duration time.Duration, years int, months int, days int, err error) {
	count := utf8.RuneCountInString(input)
	if count == 0 {
		err = errors.New("zero length input string")
		return
	}
	runes := make([]rune, count)
	for i, r := range input {
		runes[i] = r
	}
	durationString := ""
	sign := 1
	i := 0
	if runes[i] == '-' {
		durationString = "-"
		sign = -1
		i++
	} else if runes[i] == '+' {
		i++
	}
	var extraSeconds float64
	weeks := 0
	for i < count {
		// skip leading space
		for i < count {
			r := runes[i]
			if r != ' ' {
				break
			}
			i++
		}
		if i >= count {
			break
		}
		// integer part
		s := i // start index
		v := 0
		for i < count {
			r := runes[i]
			if r < '0' || r > '9' {
				break
			}
			v = v*10 + int(r-'0')
			i++
		}
		// fraction part
		d := 0.0
		if i < count && runes[i] == '.' {
			i++
			kd := 0.1
			for i < count {
				r := runes[i]
				if r < '0' || r > '9' {
					break
				}
				d += float64(r-'0') * kd
				kd *= 0.1
				i++
			}
		}
		// unit
		us := i // unit start index
		for i < count {
			r := runes[i]
			if (r >= '0' && r <= '9') || r == '.' {
				break
			}
			i++
		}
		unit := strings.Trim(string(runes[us:i]), " ")
		// verify unit
		switch unit {
		case "y":
			if years != 0 {
				err = errors.New("multiple year parts")
				return
			}
			years = sign * v
			if d > 0 {
				extraSeconds += float64(365*24*3600) * d
			}
		case "M", "mo":
			if months != 0 {
				err = errors.New("multiple month parts")
				return
			}
			months = sign * v
			if d > 0 {
				extraSeconds += float64(31*24*3600) * d // using 31-day month
			}
		case "w":
			if weeks > 0 {
				err = errors.New("multiple week parts")
				return
			}
			weeks = v
			if d > 0 {
				extraSeconds += float64(7*24*3600) * d // using 31-day month
			}
		case "d":
			if days != 0 {
				err = errors.New("multiple day parts")
				return
			}
			days = sign * v
			if d > 0 {
				extraSeconds += float64(24*3600) * d // using 31-day month
			}
		case "ns", "us", "µs", "ms", "s", "m", "h":
			durationString += string(runes[s:i])
		default:
			err = errors.New("invalid unit: " + unit)
			return
		}
	}
	if weeks > 0 {
		days += sign * weeks * 7
	}
	if len(durationString) > 1 {
		duration, err = time.ParseDuration(durationString)
		if err != nil {
			return
		}
	}
	if extraSeconds != 0 {
		duration += time.Duration(sign) * time.Duration(extraSeconds) * time.Second
	}
	return
}

func timeAddDurationString(since time.Time, durationString string) (result time.Time, err error) {
	duration, years, months, days, err := parseDuration(durationString)
	if err != nil {
		return
	}
	return since.Add(duration).AddDate(years, months, days), nil
}

func timeSubtractDurationString(since time.Time, durationString string) (result time.Time, err error) {
	duration, years, months, days, err := parseDuration(durationString)
	if err != nil {
		return
	}
	return since.Add(-duration).AddDate(-years, -months, -days), nil
}
