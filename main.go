// Command statement-check reconciles a normalized bank-statement CSV locally.
// It never uploads financial data or changes the source file.
package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"regexp"
	"sort"
	"strings"
)

type duplicate struct {
	Date        string `json:"date"`
	Description string `json:"description"`
	Amount      string `json:"amount"`
	Rows        []int  `json:"csv_rows"`
}

type report struct {
	Transactions       int         `json:"transactions"`
	OpeningBalance     string      `json:"opening_balance"`
	TransactionTotal   string      `json:"transaction_total"`
	CalculatedClosing  string      `json:"calculated_closing"`
	ExpectedClosing    string      `json:"expected_closing"`
	BalancesMatch      bool        `json:"balances_match"`
	PossibleDuplicates []duplicate `json:"possible_duplicates"`
}

func main() {
	file := flag.String("file", "", "normalized CSV with date,description,amount columns")
	opening := flag.String("opening", "", "opening balance, for example 1200.00")
	closing := flag.String("closing", "", "expected closing balance from the original statement")
	flag.Parse()
	if *file == "" || *opening == "" || *closing == "" {
		fmt.Fprintln(os.Stderr, "statement-check: -file, -opening and -closing are required")
		os.Exit(1)
	}
	result, err := reconcile(*file, *opening, *closing)
	if err != nil {
		fmt.Fprintln(os.Stderr, "statement-check:", err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		fmt.Fprintln(os.Stderr, "statement-check:", err)
		os.Exit(1)
	}
}

func reconcile(path, openingText, closingText string) (report, error) {
	opening, err := parseMoney(openingText)
	if err != nil {
		return report{}, fmt.Errorf("opening balance: %w", err)
	}
	expected, err := parseMoney(closingText)
	if err != nil {
		return report{}, fmt.Errorf("closing balance: %w", err)
	}
	file, err := os.Open(path)
	if err != nil {
		return report{}, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return report{}, fmt.Errorf("read header: %w", err)
	}
	columns := map[string]int{}
	for i, name := range headers {
		columns[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))] = i
	}
	for _, name := range []string{"date", "description", "amount"} {
		if _, ok := columns[name]; !ok {
			return report{}, fmt.Errorf("missing %q column", name)
		}
	}
	maxColumn := columns["date"]
	for _, name := range []string{"description", "amount"} {
		if columns[name] > maxColumn {
			maxColumn = columns[name]
		}
	}
	total := new(big.Int)
	seen := map[string]*duplicate{}
	count := 0
	for line := 2; ; line++ {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return report{}, fmt.Errorf("CSV row %d: %w", line, err)
		}
		if len(row) <= maxColumn {
			return report{}, fmt.Errorf("CSV row %d has too few columns", line)
		}
		date := strings.TrimSpace(row[columns["date"]])
		description := strings.TrimSpace(row[columns["description"]])
		amount, err := parseMoney(row[columns["amount"]])
		if err != nil {
			return report{}, fmt.Errorf("CSV row %d amount: %w", line, err)
		}
		if date == "" || description == "" {
			return report{}, fmt.Errorf("CSV row %d needs date and description", line)
		}
		total.Add(total, amount)
		key := date + "\x00" + description + "\x00" + amount.String()
		if seen[key] == nil {
			seen[key] = &duplicate{Date: date, Description: description, Amount: formatMoney(amount)}
		}
		seen[key].Rows = append(seen[key].Rows, line)
		count++
	}
	if count == 0 {
		return report{}, errors.New("CSV contains no transactions")
	}
	calculated := new(big.Int).Add(opening, total)
	result := report{
		Transactions: count, OpeningBalance: formatMoney(opening),
		TransactionTotal: formatMoney(total), CalculatedClosing: formatMoney(calculated),
		ExpectedClosing: formatMoney(expected), BalancesMatch: calculated.Cmp(expected) == 0,
		PossibleDuplicates: []duplicate{},
	}
	for _, group := range seen {
		if len(group.Rows) > 1 {
			result.PossibleDuplicates = append(result.PossibleDuplicates, *group)
		}
	}
	sort.Slice(result.PossibleDuplicates, func(i, j int) bool {
		return result.PossibleDuplicates[i].Rows[0] < result.PossibleDuplicates[j].Rows[0]
	})
	return result, nil
}

var moneyPattern = regexp.MustCompile(`^(?:\d+|\d{1,3}(?:,\d{3})+)(?:\.\d{1,2})?$`)

func parseMoney(value string) (*big.Int, error) {
	value = strings.TrimSpace(value)
	negative := false
	if strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")") {
		negative = true
		value = strings.TrimSpace(value[1 : len(value)-1])
	} else if strings.HasPrefix(value, "-") {
		negative = true
		value = value[1:]
	} else {
		value = strings.TrimPrefix(value, "+")
	}
	if !moneyPattern.MatchString(value) {
		return nil, fmt.Errorf("invalid amount %q; use a dot for decimals and at most two decimal places", value)
	}
	parts := strings.SplitN(strings.ReplaceAll(value, ",", ""), ".", 2)
	whole, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil, errors.New("invalid whole amount")
	}
	whole.Mul(whole, big.NewInt(100))
	if len(parts) == 2 {
		fraction := parts[1]
		if len(fraction) == 1 {
			fraction += "0"
		}
		cents, _ := new(big.Int).SetString(fraction, 10)
		whole.Add(whole, cents)
	}
	if negative {
		whole.Neg(whole)
	}
	return whole, nil
}

func formatMoney(cents *big.Int) string {
	absolute := new(big.Int).Abs(cents)
	whole, fraction := new(big.Int), new(big.Int)
	whole.QuoRem(absolute, big.NewInt(100), fraction)
	sign := ""
	if cents.Sign() < 0 {
		sign = "-"
	}
	return fmt.Sprintf("%s%s.%02d", sign, whole.String(), fraction.Int64())
}
