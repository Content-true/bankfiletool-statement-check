# Bank Statement Check

Bank Statement Check is a local command for reconciling a **normalized CSV** bank statement against opening and closing balances. It totals signed transactions and identifies exact duplicate rows for manual review. Duplicate rows are possible duplicates, not proof of an incorrect charge.

## Use

The CSV needs `date`, `description`, and `amount` columns. Expenses are negative. Amounts use a dot for decimals; commas may separate groups of three digits.

```csv
date,description,amount
2026-11-01,Deposit,120.00
2026-11-02,Payment,-45.50
```

```sh
go run . -file statement.csv -opening 1000.00 -closing 1074.50
```

The JSON report includes the calculated closing balance, whether it matches the supplied balance, and any exact duplicate date/description/amount groups. Review the original statement when a result differs. Dates and descriptions are compared as text; this tool does not interpret account ownership or classify transactions.

When using [Bank File Tool](https://bankfiletool.com/) to turn a PDF statement into a table, first inspect several exported rows against the PDF, then run this check on a normalized CSV. This command does not convert PDFs, call Bank File Tool, or upload your financial data. Keep sensitive files on a trusted device and follow your organization's handling policy.

## Requirements

Go 1.22 or newer. No third-party Go dependencies.

## License

MIT; see [LICENSE](LICENSE).
