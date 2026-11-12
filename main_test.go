package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReconcileAndDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "statement.csv")
	csv := "date,description,amount\n2026-11-01,Deposit,10.00\n2026-11-02,Fee,(1.25)\n2026-11-02,Fee,(1.25)\n"
	if err := os.WriteFile(path, []byte(csv), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := reconcile(path, "1,000.00", "1007.50")
	if err != nil {
		t.Fatal(err)
	}
	if !r.BalancesMatch || r.TransactionTotal != "7.50" || len(r.PossibleDuplicates) != 1 || len(r.PossibleDuplicates[0].Rows) != 2 {
		t.Fatalf("unexpected report: %+v", r)
	}
}

func TestRejectFractionalCents(t *testing.T) {
	if _, err := parseMoney("1.234"); err == nil {
		t.Fatal("expected error for three decimal places")
	}
}
